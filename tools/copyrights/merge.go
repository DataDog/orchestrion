// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

func main() {
	var (
		licensesDir string
		outputFile  string
	)
	flag.StringVar(&licensesDir, "licenses", "/tmp/licenses", "directory containing all pages license information")
	flag.StringVar(&outputFile, "output", "LICENSE-3rdparty.csv", "output file to write")
	flag.Parse()

	licensesDir = path.Clean(licensesDir)
	outputFile = path.Clean(outputFile)

	var store Licenses
	for _, filename := range flag.Args() {
		log.Printf("Loading data from %s\n", filename)
		if err := store.LoadFile(filename); err != nil {
			log.Fatalf("Failed to load %q: %v\n", filename, err)
		}
	}

	log.Println("Adding copyright information")
	store.AddCopyrights(licensesDir)

	log.Printf("Writing data to %s\n", outputFile)
	if err := store.WriteFile(outputFile); err != nil {
		log.Fatalf("Error writing %q: %v\n", outputFile, err)
	}
}

type (
	Licenses struct {
		data map[string]map[string]*License
	}
	License struct {
		spdx      string
		copyright string
	}
)

func (l *Licenses) LoadFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}
	var (
		componentIndex = -1
		originIndex    = -1
		licenseIndex   = -1
	)
	for i, name := range records[0] {
		switch name {
		case "Component":
			componentIndex = i
		case "Origin":
			originIndex = i
		case "License":
			licenseIndex = i
		}
	}
	if componentIndex == -1 {
		return fmt.Errorf("missing Component header in %q", records[0])
	}
	if originIndex == -1 {
		return fmt.Errorf("missing Origin header in %q", records[0])
	}
	if licenseIndex == -1 {
		return fmt.Errorf("missing License header in %q", records[0])
	}

	for _, record := range records[1:] {
		component := record[componentIndex]
		origin := record[originIndex]
		license := record[licenseIndex]

		if l.data == nil {
			l.data = make(map[string]map[string]*License)
		}
		compLicense := l.data[component]
		if compLicense == nil {
			compLicense = make(map[string]*License)
			l.data[component] = compLicense
		}
		if licenseInfo, found := compLicense[origin]; found {
			if licenseInfo.spdx != license {
				return fmt.Errorf("conflicting licence name for %s: %s and %s", origin, licenseInfo.spdx, license)
			}
		} else {
			compLicense[origin] = &License{spdx: license}
		}
	}

	return nil
}

func (l *Licenses) AddCopyrights(pkgDir string) {
	for _, compData := range l.data {
		for origin, license := range compData {
			license.copyright = scanPkg(path.Join(pkgDir, origin))
		}
	}
}

func (l *Licenses) WriteFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"Component", "Origin", "License", "Copyright"}); err != nil {
		return err
	}

	var records [][4]string
	for component, compData := range l.data {
		for origin, license := range compData {
			records = append(records, [4]string{component, origin, license.spdx, license.copyright})
		}
	}
	sort.Slice(records, func(l, r int) bool {
		ld := records[l]
		rd := records[r]
		for i := 0; i < 4; i++ {
			cmp := strings.Compare(ld[i], rd[i])
			if cmp < 0 {
				return true
			} else if cmp > 0 {
				return false
			}
		}
		return false
	})

	for _, record := range records {
		if err := w.Write(record[:]); err != nil {
			return err
		}
	}

	return nil
}

func scanPkg(pkg string) string {
	entries, err := os.ReadDir(pkg)
	if err != nil {
		log.Printf("warn: skipping %s because of error %s", pkg, err)
		return "unknown"
	}
	var (
		copyrights []string
		dedup      map[string]struct{}
	)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		c := scanFile(path.Join(pkg, entry.Name()))
		for _, c := range c {
			if _, dup := dedup[c]; dup {
				continue
			}
			copyrights = append(copyrights, c)
		}
	}
	if len(copyrights) > 0 {
		return strings.Join(copyrights, " | ")
	}
	return "unknown"
}

var hasDigits = regexp.MustCompile(`\d`)

func isCopyright(line string) (string, bool) {
	line = strings.TrimSpace(line)
	return line, strings.HasPrefix(line, "Copyright") && hasDigits.MatchString(line)
}

func scanFile(fname string) []string {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("cannot open %s: %s", fname, err)
	}
	defer f.Close()
	var copyrights []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if l, ok := isCopyright(line); ok {
			copyrights = append(copyrights, l)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return copyrights
}
