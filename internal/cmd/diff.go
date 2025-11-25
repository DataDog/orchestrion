// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/DataDog/orchestrion/internal/binpath"
	"github.com/DataDog/orchestrion/internal/goproxy"
	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/DataDog/orchestrion/internal/report"
	"github.com/urfave/cli/v2"
)

var (
	filenameFlag = cli.BoolFlag{
		Name:  "files",
		Usage: "Only show file paths created by orchestrion instead of diff output",
	}

	filterFlag = cli.StringFlag{
		Name:  "filter",
		Usage: "Filter the diff to a regex matched on the package or file paths from the build.",
	}

	packageFlag = cli.BoolFlag{
		Name:  "package",
		Usage: "Print package names instead of printing the diff",
	}

	debugFlag = cli.BoolFlag{
		Name:  "debug",
		Usage: "Also print synthetic and tracer weaved packages",
	}

	buildFlag = cli.BoolFlag{
		Name:  "build",
		Usage: "Execute a build with -work before generating the diff. All remaining arguments after flags are passed to the build command.",
	}

	noCacheFlag = cli.BoolFlag{
		Name:  "no-cache",
		Usage: "Force a rebuild of all packages when using --build (adds -a flag). Useful for ensuring complete instrumentation coverage.",
	}

	Diff = &cli.Command{
		Name:  "diff",
		Usage: "Generates a diff between a nominal and orchestrion-instrumented build. Use --build to execute a build first, or provide a work directory path obtained from `orchestrion go build -work -a`. This is incompatible with coverage related flags.",
		Args:  true,
		Flags: []cli.Flag{
			&filenameFlag,
			&filterFlag,
			&packageFlag,
			&debugFlag,
			&buildFlag,
			&noCacheFlag,
		},
		Action: func(clictx *cli.Context) error {
			workFolder, err := workFolder(clictx)
			if err != nil {
				return err
			}

			report, err := report.FromWorkDir(clictx.Context, workFolder)
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed to read work dir: %s (did you forgot the -work flag during build ?)", err), 1)
			}

			if report.IsEmpty() {
				return cli.Exit("no files to diff (did you forgot the -a flag during build?)", 1)
			}

			if !clictx.Bool(debugFlag.Name) {
				report = report.WithSpecialCasesFilter()
			}

			if filter := clictx.String(filterFlag.Name); filter != "" {
				report, err = report.WithRegexFilter(filter)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to filter files: %s", err), 1)
				}
			}

			return outputReport(clictx, report)
		},
	}
)

func workFolder(clictx *cli.Context) (string, error) {
	if !clictx.Bool(buildFlag.Name) {
		workFolder := clictx.Args().First()
		if workFolder == "" {
			return "", cli.ShowSubcommandHelp(clictx)
		}
		return workFolder, nil
	}

	return executeBuildAndCaptureWorkDir(clictx, prepareBuildArgs(clictx.Args().Slice(), clictx.Bool(noCacheFlag.Name)))
}

func prepareBuildArgs(args []string, forceRebuild bool) []string {
	switch {
	case len(args) == 0:
		args = []string{"build", "./..."}
	case args[0] != "build" && args[0] != "install" && args[0] != "test":
		args = append([]string{"build"}, args...)
	}

	var flags []string
	hasWork, hasAll := false, false
	for _, arg := range args {
		switch arg {
		case "-work":
			hasWork = true
		case "-a":
			hasAll = true
		}
	}

	if !hasWork {
		flags = append(flags, "-work")
	}
	if !hasAll && forceRebuild {
		flags = append(flags, "-a")
	}

	if len(flags) > 0 {
		args = slices.Concat(args[:1], flags, args[1:])
	}

	return args
}

func outputReport(clictx *cli.Context, rpt report.Report) error {
	if clictx.Bool(packageFlag.Name) {
		for _, pkg := range rpt.Packages() {
			_, _ = fmt.Fprintln(clictx.App.Writer, pkg)
		}
		return nil
	}

	if clictx.Bool(filenameFlag.Name) {
		for _, file := range rpt.Files() {
			_, _ = fmt.Fprintln(clictx.App.Writer, file)
		}
		return nil
	}

	if err := rpt.Diff(clictx.App.Writer); err != nil {
		return cli.Exit(fmt.Sprintf("failed to generate diff: %s", err), 1)
	}

	return nil
}

func executeBuildAndCaptureWorkDir(clictx *cli.Context, buildArgs []string) (string, error) {
	if err := pin.AutoPinOrchestrion(clictx.Context, clictx.App.Writer, clictx.App.ErrWriter); err != nil {
		return "", cli.Exit(err, -1)
	}

	cmd, err := goproxy.BuildCmd(clictx.Context, buildArgs, goproxy.WithToolexec(binpath.Orchestrion, "toolexec"))
	if err != nil {
		return "", fmt.Errorf("building command: %w", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("creating pipe: %w", err)
	}
	defer r.Close()
	defer w.Close()

	var workDirBuffer strings.Builder
	teeReader := io.TeeReader(r, io.MultiWriter(clictx.App.ErrWriter, &workDirBuffer))

	cmd.Stderr = w

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := io.Copy(io.Discard, teeReader)
		if err != nil {
			_, _ = fmt.Fprintf(clictx.App.ErrWriter, "failed to read build output: %v", err)
		}
	}()

	buildErr := cmd.Run()

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("closing pipe writer: %w", err)
	}
	<-done

	if buildErr != nil {
		return "", cli.Exit(fmt.Sprintf("build failed: %v", buildErr), 1)
	}

	workDir := extractWorkDirFromOutput(workDirBuffer.String())
	if workDir == "" {
		return "", cli.Exit("could not extract work directory from build output (did the build use -work?)", 1)
	}
	return workDir, nil
}

func extractWorkDirFromOutput(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if wd, ok := strings.CutPrefix(strings.TrimSpace(line), "WORK="); ok {
			return wd
		}
	}
	return ""
}
