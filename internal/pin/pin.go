// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DataDog/orchestrion/internal/ensure"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/dave/jennifer/jen"
	"golang.org/x/term"
)

const (
	orchestrionImportPath = "github.com/DataDog/orchestrion"
	orchestrionToolGo     = "orchestrion.tool.go"
	envVarCheckedGoMod    = "DD_ORCHESTRION_IS_GOMOD_VERSION"
	envValTrue            = "true"
)

var (
	requiredVersionError error // Whether the go.mod version check succeeded
)

// AutoPinOrchestrion automatically runs `pinOrchestrion` if the necessary
// requirements are not already met. It prints messages to `os.Stderr` to inform
// the user about what is going on.
func AutoPinOrchestrion() {
	if requiredVersionError == nil {
		// Nothing to do!
		return
	}

	var (
		box       = lipgloss.NewStyle()
		stylePath = lipgloss.NewStyle()
		styleFile = lipgloss.NewStyle()
		styleCmd  = lipgloss.NewStyle()
	)
	if term.IsTerminal(int(os.Stderr.Fd())) {
		box = box.Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.ANSIColor(1)).
			Padding(1, 2)
		if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil {
			box.Width(w - box.GetHorizontalMargins() - box.GetHorizontalBorderSize())
		}

		stylePath = stylePath.Foreground(lipgloss.ANSIColor(4)).Underline(true)
		styleFile = styleFile.Foreground(lipgloss.ANSIColor(2)).Underline(true)
		styleCmd = styleCmd.Foreground(lipgloss.ANSIColor(5)).Bold(true).Underline(true)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Version check error: %v\n", requiredVersionError)
	}

	var builder strings.Builder
	_, _ = builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(3)).Render("Warning:"))
	_, _ = builder.WriteRune(' ')
	_, _ = builder.WriteString(stylePath.Render(orchestrionImportPath))
	_, _ = builder.WriteString(" is not present in your ")
	_, _ = builder.WriteString(styleFile.Render("go.mod"))
	_, _ = builder.WriteString(" file.\nIn order to ensure build reliability and reproductibility, orchestrion")
	_, _ = builder.WriteString(" will now add itself in your ")
	_, _ = builder.WriteString(styleFile.Render("go.mod"))
	_, _ = builder.WriteString(" file by:\n\n\t1. creating a new file named ")
	_, _ = builder.WriteString(styleFile.Render(orchestrionToolGo))
	_, _ = builder.WriteString("\n\t2. running ")
	_, _ = builder.WriteString(styleCmd.Render(fmt.Sprintf("go get %s@%s", orchestrionImportPath, version.Tag)))
	_, _ = builder.WriteString("\n\t3. running ")
	_, _ = builder.WriteString(styleCmd.Render("go mod tidy"))
	_, _ = builder.WriteString("\n\nYou should commit the resulting changes into your source control system.")

	message := builder.String()
	_, _ = fmt.Fprintln(os.Stderr, box.Render(message))

	if err := PinOrchestrion(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to pin orchestrion in go.mod: %v\n", err)
		os.Exit(1)
	}

	requiredVersionError = nil
}

func PinOrchestrion() error {
	goMod, err := goenv.GOMOD()
	if err != nil {
		return fmt.Errorf("getting GOMOD: %w", err)
	}

	code := jen.NewFile("tools")
	code.HeaderComment(strings.Join([]string{
		"// Code generated by `orchestrion pin`; DO NOT EDIT.",
		"",
		"// This file is generated by `orchestrion pin`, and is used to include a blank import of the",
		"// orchestrion package(s) so that `go mod tidy` does not remove the requirements from go.mod.",
		"// This file should be checked into source control.",
	}, "\n"))
	code.PackageComment("//go:build tools")
	code.Anon(orchestrionImportPath)

	// We write into a temporary file, and then rename it in place. This reduces the risk of
	// concurrent calls resulting in partial writes, etc...
	toolFile := filepath.Join(goMod, "..", orchestrionToolGo)
	tmpFile, err := os.CreateTemp(filepath.Dir(toolFile), "orchestrion.tool.go.*")
	if err != nil {
		return fmt.Errorf("creating temporary %q: %w", tmpFile.Name(), err)
	}
	err = code.Render(tmpFile)
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing %q: %w", tmpFile.Name(), err)
	}
	if err != nil {
		return fmt.Errorf("writing to %q: %w", tmpFile.Name(), err)
	}
	if err = os.Rename(tmpFile.Name(), toolFile); err != nil {
		return fmt.Errorf("renaming %q to %q: %w", tmpFile.Name(), toolFile, err)
	}

	pkgVersion := fmt.Sprintf("%s@%s", orchestrionImportPath, version.Tag)
	if err := exec.Command("go", "get", pkgVersion).Run(); err != nil {
		return fmt.Errorf("running `go get %s`: %w", pkgVersion, err)
	}

	if err := exec.Command("go", "mod", "tidy").Run(); err != nil {
		return fmt.Errorf("running `go mod tidy`: %w", err)
	}

	return nil
}

func init() {
	if os.Getenv(envVarCheckedGoMod) == envValTrue {
		// A parent process has already done the check for us!!
		return
	}

	if requiredVersionError = ensure.RequiredVersion(); requiredVersionError != nil {
		log.Tracef("Failed to detect required version of orchestrion from go.mod: %v\n", requiredVersionError)
		if wd, err := os.Getwd(); err == nil {
			log.Tracef("Working directory: %q\n", wd)
		}
		log.Tracef("GOMOD=%s\n", os.Getenv("GOMOD"))
	} else {
		_ = os.Setenv(envVarCheckedGoMod, envValTrue)
	}
}
