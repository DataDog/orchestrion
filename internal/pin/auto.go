// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog"
	"golang.org/x/term"

	"github.com/DataDog/orchestrion/internal/ensure"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
)

const envVarCheckedGoMod = "DD_ORCHESTRION_IS_GOMOD_VERSION"

// AutoPinOrchestrion automatically runs [PinOrchestrion] if the necessary
// requirements are not already met. It prints messages to `stderr` to inform
// the user about what is going on.
func AutoPinOrchestrion(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	log := zerolog.Ctx(ctx)

	if os.Getenv(envVarCheckedGoMod) == "true" {
		// A parent process (or ourselves earlier) has already done the check
		return nil
	}

	// Make sure we don't do this again
	if err := os.Setenv(envVarCheckedGoMod, "true"); err != nil {
		return fmt.Errorf("os.Setenv("+envVarCheckedGoMod+", true): %w", err)
	}

	if _, isDev := version.TagInfo(); !isDev {
		err := ensure.RequiredVersion(ctx)
		if errors.As(err, &ensure.IncorrectVersionError{}) {
			// There is already a required version, but we're not running that one!
			log.Trace().Err(err).Msg("Orchestrion is already in go.mod, but we are not running the correct one; returning an error")
			return err
		}
		if err == nil {
			// We're good to go
			log.Trace().Msg("Orchestrion is already in go.mod, and we are running the correct version, no automatic pinning required")
			return nil
		}
		log.Trace().Err(err).Msg("Failed to detect required version of orchestrion from go.mod")
	} else {
		log.Trace().Msg("Skipping ensure.RequiredVersion because this is a development build")
		_, err := os.Stat(config.FilenameOrchestrionToolGo)
		if err == nil {
			log.Trace().Msg("Found " + config.FilenameOrchestrionToolGo + " file, no automatic pinning required")
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			log.Trace().Err(err).Msg("Failed to stat " + config.FilenameOrchestrionToolGo + ", returning error")
			return err
		}
		log.Trace().Msg("No " + config.FilenameOrchestrionToolGo + " file found, will attempt automatic pinning")
	}

	var (
		box       = lipgloss.NewStyle()
		stylePath = lipgloss.NewStyle()
		styleFile = lipgloss.NewStyle()
		styleCmd  = lipgloss.NewStyle()
	)
	if stderr, isFile := stderr.(*os.File); isFile && term.IsTerminal(int(stderr.Fd())) {
		box = box.Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.ANSIColor(1)).
			Padding(1, 2)
		if w, _, err := term.GetSize(int(stderr.Fd())); err == nil {
			box.Width(w - box.GetHorizontalMargins() - box.GetHorizontalBorderSize())
		}

		stylePath = stylePath.Foreground(lipgloss.ANSIColor(4)).Underline(true)
		styleFile = styleFile.Foreground(lipgloss.ANSIColor(2)).Underline(true)
		styleCmd = styleCmd.Foreground(lipgloss.ANSIColor(5)).Bold(true).Underline(true)
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
	_, _ = fmt.Fprintln(stderr, box.Render(message))

	if err := PinOrchestrion(ctx, Options{Writer: stdout, ErrWriter: stderr}); err != nil {
		return fmt.Errorf("failed to pin orchestrion in go.mod: %w", err)
	}

	return nil
}
