// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DataDog/orchestrion/internal/ensure"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog"
	"golang.org/x/term"
)

const envVarCheckedGoMod = "DD_ORCHESTRION_IS_GOMOD_VERSION"

// AutoPinOrchestrion automatically runs `pinOrchestrion` if the necessary
// requirements are not already met. It prints messages to `os.Stderr` to inform
// the user about what is going on.
func AutoPinOrchestrion(ctx context.Context) {
	log := zerolog.Ctx(ctx)

	if os.Getenv(envVarCheckedGoMod) == "true" {
		// A parent process (or ourselves earlier) has already done the check
		return
	}

	// Make sure we don't do this again
	defer func() {
		_ = os.Setenv(envVarCheckedGoMod, "true")
	}()

	var requiredVersionError error
	if _, isDev := version.TagInfo(); !isDev {
		requiredVersionError = ensure.RequiredVersion(ctx)
		if requiredVersionError == nil {
			// We're good to go
			return
		}
	} else {
		log.Trace().Msg("Skipping ensure.RequiredVersion because this is a development build")
		_, err := os.Stat(config.FilenameOrchestrionToolGo)
		if err == nil {
			log.Trace().Msg("Found " + config.FilenameOrchestrionToolGo + " file, no automatic pinning required")
			return
		}
		requiredVersionError = fmt.Errorf("stat %s: %w", config.FilenameOrchestrionToolGo, err)
	}

	log.Trace().Err(requiredVersionError).Msg("Failed to detect required version of orchestrion from go.mod")

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
	_, _ = builder.WriteString(styleFile.Render(config.FilenameOrchestrionToolGo))
	_, _ = builder.WriteString("\n\t2. running ")
	rawTag, _ := version.TagInfo()
	_, _ = builder.WriteString(styleCmd.Render(fmt.Sprintf("go get %s@%s", orchestrionImportPath, rawTag)))
	_, _ = builder.WriteString("\n\t3. running ")
	_, _ = builder.WriteString(styleCmd.Render("go mod tidy"))
	_, _ = builder.WriteString("\n\nYou should commit the resulting changes into your source control system.")

	message := builder.String()
	_, _ = fmt.Fprintln(os.Stderr, box.Render(message))

	if err := PinOrchestrion(ctx, Options{}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to pin orchestrion in go.mod: %v\n", err)
		os.Exit(1)
	}
}
