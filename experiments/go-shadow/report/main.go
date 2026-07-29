// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Command report renders experiments/go-shadow/COVERAGE-LEDGER.md as a single
// self-contained HTML page.
//
// The ledger is the source of truth; this command only presents it, so the report can
// never disagree with the recorded evidence. It also recomputes every verdict tally
// from the matrix rows rather than copying the ledger's summary, which makes a stale
// summary visible as a mismatch instead of propagating it.
//
// Usage:
//
//	go run ./experiments/go-shadow/report -o stock-vs-orchestrion.html
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type verdict string

const (
	verdictWin        verdict = "Win"
	verdictLoss       verdict = "Loss"
	verdictPartial    verdict = "Partial"
	verdictNotProven  verdict = "Not proven"
	verdictUnclassifi verdict = "Other"
)

var verdictOrder = []verdict{verdictWin, verdictLoss, verdictPartial, verdictNotProven}

// cell is one prototype's verdict for one case.
type cell struct {
	Verdict  verdict
	Measured bool
	Evidence template.HTML
}

// row is one capability case, compared across both prototypes.
type row struct {
	ID          int
	Case        string
	Section     string
	PatchedGo   cell
	Orchestrion cell
}

// tally counts verdicts for one prototype column.
type tally struct {
	Column   string
	Counts   map[verdict]int
	Measured int
	Total    int
	LossIDs  []int
}

type section struct {
	Title string
	Rows  []row
}

type notProven struct {
	ID          int
	Case        string
	PatchedGo   string
	Orchestrion string
	Why         template.HTML
}

type reportData struct {
	Title       string
	Intro       template.HTML
	Callouts    []template.HTML
	Sections    []section
	Tallies     []tally
	NotProven   []notProven
	HowToRun    string
	SourcePath  string
	TotalCases  int
	TotalCells  int
	MeasuredAny int
}

func main() {
	var ledgerPath, outputPath string
	flag.StringVar(&ledgerPath, "ledger", "", "path to COVERAGE-LEDGER.md (default: sibling of this command)")
	flag.StringVar(&outputPath, "o", "stock-vs-orchestrion.html", "output HTML path")
	flag.Parse()

	if ledgerPath == "" {
		ledgerPath = filepath.Join("experiments", "go-shadow", "COVERAGE-LEDGER.md")
	}

	contents, err := os.ReadFile(ledgerPath)
	if err != nil {
		fatal("read ledger: %v", err)
	}

	data, err := parseLedger(string(contents), ledgerPath)
	if err != nil {
		fatal("parse ledger: %v", err)
	}

	rendered, err := render(data)
	if err != nil {
		fatal("render report: %v", err)
	}

	if err := os.WriteFile(outputPath, rendered, 0o644); err != nil {
		fatal("write report: %v", err)
	}

	fmt.Printf("wrote %s (%d bytes)\n", outputPath, len(rendered))
	for _, t := range data.Tallies {
		fmt.Printf("  %-14s win=%d loss=%d partial=%d not-proven=%d measured=%d total=%d\n",
			t.Column, t.Counts[verdictWin], t.Counts[verdictLoss], t.Counts[verdictPartial],
			t.Counts[verdictNotProven], t.Measured, t.Total)
	}
	fmt.Printf("  cases=%d measured cells=%d cases with a measured cell=%d\n",
		data.TotalCases, data.TotalCells, data.MeasuredAny)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "report: "+format+"\n", args...)
	os.Exit(1)
}

var (
	sectionPattern    = regexp.MustCompile(`^###\s+(.*)$`)
	headingPattern    = regexp.MustCompile(`^##\s+(.*)$`)
	numericIDPattern  = regexp.MustCompile(`^\d+$`)
	inlineCodePattern = regexp.MustCompile("`([^`]*)`")
	boldPattern       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicPattern     = regexp.MustCompile(`\*([^*]+)\*`)
	fenceMarker       = "```"
)

func parseLedger(contents, ledgerPath string) (reportData, error) {
	data := reportData{
		Title:      "Go IAST taint tracking - patched Go vs Orchestrion",
		SourcePath: ledgerPath,
	}

	lines := strings.Split(contents, "\n")

	var (
		currentHeading string
		currentSection string
		intro          []string
		callout        []string
		howToRun       []string
		inFence        bool
		sections       []section
		sectionIndex   = map[string]int{}
		notProvenRows  []notProven
	)

	appendRow := func(sectionTitle string, r row) {
		index, ok := sectionIndex[sectionTitle]
		if !ok {
			sections = append(sections, section{Title: sectionTitle})
			index = len(sections) - 1
			sectionIndex[sectionTitle] = index
		}
		sections[index].Rows = append(sections[index].Rows, r)
	}

	flushCallout := func() {
		if len(callout) == 0 {
			return
		}
		data.Callouts = append(data.Callouts, inline(strings.Join(callout, " ")))
		callout = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, fenceMarker) {
			inFence = !inFence
			continue
		}
		if inFence {
			if currentHeading == "How to run" {
				howToRun = append(howToRun, line)
			}
			continue
		}

		if match := sectionPattern.FindStringSubmatch(trimmed); match != nil {
			flushCallout()
			currentSection = match[1]
			continue
		}
		if match := headingPattern.FindStringSubmatch(trimmed); match != nil {
			flushCallout()
			currentHeading = match[1]
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			if text == "" {
				flushCallout()
				continue
			}
			callout = append(callout, text)
			continue
		}
		flushCallout()

		if strings.HasPrefix(trimmed, "|") {
			cells := splitTableRow(trimmed)
			switch {
			case len(cells) == 6 && numericIDPattern.MatchString(cells[0]):
				id, err := strconv.Atoi(cells[0])
				if err != nil {
					return data, fmt.Errorf("case id %q: %w", cells[0], err)
				}
				appendRow(currentSection, row{
					ID:          id,
					Case:        cells[1],
					Section:     currentSection,
					PatchedGo:   parseCell(cells[2], cells[3]),
					Orchestrion: parseCell(cells[4], cells[5]),
				})
			case len(cells) == 5 && numericIDPattern.MatchString(cells[0]) && currentHeading == "Remaining not proven":
				id, err := strconv.Atoi(cells[0])
				if err != nil {
					return data, fmt.Errorf("not-proven id %q: %w", cells[0], err)
				}
				notProvenRows = append(notProvenRows, notProven{
					ID:          id,
					Case:        cells[1],
					PatchedGo:   stripEmphasis(cells[2]),
					Orchestrion: stripEmphasis(cells[3]),
					Why:         inline(cells[4]),
				})
			}
			continue
		}

		if currentHeading == "" && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			intro = append(intro, trimmed)
		}
	}
	flushCallout()

	if len(sections) == 0 {
		return data, fmt.Errorf("no matrix rows found; the ledger table shape changed")
	}

	data.Intro = inline(strings.Join(intro, " "))
	data.Sections = sections
	data.NotProven = notProvenRows
	data.HowToRun = strings.TrimSpace(strings.Join(howToRun, "\n"))

	patched := tally{Column: "Patched Go", Counts: map[verdict]int{}}
	orchestrion := tally{Column: "Orchestrion", Counts: map[verdict]int{}}
	for _, s := range data.Sections {
		for _, r := range s.Rows {
			data.TotalCases++
			accumulate(&patched, r.ID, r.PatchedGo)
			accumulate(&orchestrion, r.ID, r.Orchestrion)
			if r.PatchedGo.Measured || r.Orchestrion.Measured {
				data.MeasuredAny++
			}
		}
	}
	data.TotalCells = patched.Measured + orchestrion.Measured
	sort.Ints(patched.LossIDs)
	sort.Ints(orchestrion.LossIDs)
	data.Tallies = []tally{patched, orchestrion}

	return data, nil
}

func accumulate(t *tally, id int, c cell) {
	t.Total++
	t.Counts[c.Verdict]++
	if c.Measured {
		t.Measured++
	}
	if c.Verdict == verdictLoss {
		t.LossIDs = append(t.LossIDs, id)
	}
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, len(parts))
	for i, part := range parts {
		cells[i] = strings.TrimSpace(part)
	}
	return cells
}

func parseCell(verdictText, evidenceText string) cell {
	measured := strings.Contains(verdictText, "✎")
	name := stripEmphasis(strings.ReplaceAll(verdictText, "✎", ""))

	resolved := verdictUnclassifi
	for _, candidate := range verdictOrder {
		if strings.EqualFold(name, string(candidate)) {
			resolved = candidate
			break
		}
	}

	return cell{Verdict: resolved, Measured: measured, Evidence: inline(evidenceText)}
}

func stripEmphasis(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "*", ""))
}

// inline renders the small markdown subset the ledger actually uses: inline code spans,
// bold runs and italic runs. Everything is HTML-escaped first, so evidence text can never
// inject markup; only the recognised markers are turned back into elements. Bold is
// substituted before italic so `**x**` does not degrade into a nested emphasis.
func inline(text string) template.HTML {
	escaped := template.HTMLEscapeString(text)
	escaped = inlineCodePattern.ReplaceAllString(escaped, "<code>$1</code>")
	escaped = boldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = italicPattern.ReplaceAllString(escaped, "<em>$1</em>")
	return template.HTML(escaped)
}

func (c cell) Class() string {
	switch c.Verdict {
	case verdictWin:
		return "win"
	case verdictLoss:
		return "loss"
	case verdictPartial:
		return "partial"
	case verdictNotProven:
		return "unproven"
	default:
		return "other"
	}
}

func (t tally) LossList() string {
	if len(t.LossIDs) == 0 {
		return "none"
	}
	parts := make([]string, len(t.LossIDs))
	for i, id := range t.LossIDs {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ", ")
}

func (t tally) Count(name verdict) int { return t.Counts[name] }

func (t tally) ClassOf(name verdict) string {
	return cell{Verdict: name}.Class()
}

func (t tally) Percent(name verdict) string {
	if t.Total == 0 {
		return "0"
	}
	return strconv.FormatFloat(100*float64(t.Counts[name])/float64(t.Total), 'f', 1, 64)
}

func render(data reportData) ([]byte, error) {
	functions := template.FuncMap{
		"verdicts": func() []verdict { return verdictOrder },
		"anchor": func(title string) string {
			lowered := strings.ToLower(title)
			var builder strings.Builder
			for _, r := range lowered {
				switch {
				case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
					builder.WriteRune(r)
				default:
					builder.WriteRune('-')
				}
			}
			return strings.Trim(builder.String(), "-")
		},
	}

	tmpl, err := template.New("report").Funcs(functions).Parse(reportTemplate)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
