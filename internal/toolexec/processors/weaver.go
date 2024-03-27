// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/rogpeppe/go-internal/lockedfile"
)

const (
	dotLock     = ".lock"
	dotOriginal = ".original"
)

type AspectWeaver struct{}

func (w *AspectWeaver) ProcessAsm(cmd *proxy.AsmCommand) error {
	defer w.redirectLogger(cmd.StageDir, "asm")()
	log.Printf("Original command: %q\n", cmd.Args())

	_, owned, err := IndexCompileUnit(cmd.Flags.Package, cmd.StageDir)
	if err != nil {
		return fmt.Errorf("failed indexing compile unit: %w", err)
	}

	if owned {
		return nil
	}

	log.Println("Re-using build output from another process, replacing all source files with empty objects to avoid duplicated symbols...")
	srcDir := path.Join(cmd.StageDir, "src")
	blankFile := path.Join(srcDir, "blank.s")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return fmt.Errorf("failed creating generated source directory: %w", err)
	}
	if err := os.WriteFile(blankFile, []byte{}, 0o644); err != nil {
		return fmt.Errorf("failed creating blank source file: %w", err)
	}
	for _, file := range cmd.SourceFiles() {
		if err := cmd.ReplaceParam(file, blankFile); err != nil {
			return fmt.Errorf("failed replacing argument %q: %w", file, err)
		}
	}

	return nil
}

func (w *AspectWeaver) ProcessCgo(cmd *proxy.CgoCommand) error {
	defer w.redirectLogger(cmd.StageDir, "cgo")()
	log.Printf("Original command: %q\n", cmd.Args())

	// NOTE: We have to use `TOOLEXEC_IMPORTPATH` here because one variant of cgo only has the package name, not the full
	// import path...
	linkname, owned, err := IndexCompileUnit(os.Getenv("TOOLEXEC_IMPORTPATH"), cmd.StageDir)
	if err != nil {
		return fmt.Errorf("failed indexing compile unit: %w", err)
	}

	if owned {
		return nil
	}

	log.Println("Re-using build output from another process, creatig blank objects & skipping command...")
	entries, err := os.ReadDir(linkname)
	if err != nil {
		return fmt.Errorf("error listing directory %q: %w", linkname, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var data []byte
		switch ext := path.Ext(entry.Name()); ext {
		case ".c":
		case ".go":
			buf := bytes.NewBuffer(nil)
			pkg := cmd.Flags.DynImport
			if pkg == "" {
				parts := strings.Split(cmd.Flags.ImportPath, "/")
				pkg = parts[len(parts)-1]
			}
			fmt.Fprintf(buf, "package %s\n\n// Intentionally blank", pkg)
			data = buf.Bytes()
		default:
			continue
		}
		blank := path.Join(cmd.StageDir, path.Base(entry.Name()))
		log.Printf("Creating blank stand-in at %q\n", blank)
		if err := os.WriteFile(blank, data, 0o644); err != nil {
			return fmt.Errorf("error creating blank file %q: %w", blank, err)
		}
	}

	return proxy.ErrSkipCommand
}

func (w *AspectWeaver) ProcessCompile(cmd *proxy.CompileCommand) error {
	defer w.redirectLogger(cmd.StageDir, "compile")()
	log.Printf("Original command: %q\n", cmd.Args())

	goFiles := cmd.GoFiles()

	linkname, owned, err := IndexCompileUnit(cmd.Flags.Package, cmd.StageDir)
	if err != nil {
		return fmt.Errorf("failed indexing compile unit: %w", err)
	} else if !owned {
		relpath, err := filepath.Rel(cmd.StageDir, cmd.Flags.Output)
		if err != nil {
			return fmt.Errorf("could not compute relative path of %q in %q: %w", cmd.Flags.Output, cmd.StageDir, err)
		}

		lockFile := path.Join(linkname, relpath) + dotLock
		if _, err := waitUntilNonEmpty(lockFile, time.Minute); err != nil {
			return fmt.Errorf("failure waiting for read lock: %w", err)
		}

		log.Println("Read lock acquired, linking previoud compilation results into place...")
		return linkCompileOutput(cmd, linkname)
	}

	log.Printf("Acquiring %q for writing...", cmd.Flags.Output+dotLock)
	if lockedFile, err := lockedfile.Create(cmd.Flags.Output + dotLock); err != nil {
		return fmt.Errorf("failed to create lock file %q: %w", lockedFile.Name(), err)
	} else {
		// Write something in the file so that the other processes can tell if they're holding the lock
		// for real, or arrived before it was write-locked by this process.
		fmt.Fprintf(lockedFile, "%d", os.Getpid())
		cmd.OnClose(func() error {
			log.Printf("Releasing exclusive lock on %q...\n", lockedFile.Name())
			return lockedFile.Close()
		})
	}

	if strings.HasPrefix(cmd.Flags.Package, "github.com/datadog/orchestrion/") ||
		strings.HasPrefix(cmd.Flags.Package, "gopkg.in/DataDog/dd-trace-go.v1/") {
		// Don't instrument the instrumentation itself...
		log.Printf("Skipping aspects weaving in %q: as it is an instruments package...\n", cmd.Flags.Package)
		return nil
	}

	log.Printf("Weaving aspects in %q...\n", cmd.Flags.Package)
	defer log.Printf("Done weaving aspects in %q!\n", cmd.Flags.Package)

	inj, err := injector.New(
		cmd.BuildDir,
		injector.Options{
			Aspects:          builtin.Aspects[:],
			PreserveLineInfo: true,
			ModifiedFile:     func(name string) string { return path.Join(cmd.StageDir, "src", path.Base(name)) },
		},
	)
	if err != nil {
		return err
	}

	var references typed.ReferenceMap
	for _, file := range goFiles {
		res, err := inj.InjectFile(file, map[string]string{"httpmode": "wrap"})
		if err != nil {
			return fmt.Errorf("error while weaving %q: %w", file, err)
		}
		if res.Modified {
			log.Printf("Replacing source file %q with woven copy %q\n", file, res.Filename)
			if err := cmd.ReplaceParam(file, res.Filename); err != nil {
				return err
			}
			references.Merge(res.References)
		}
	}

	if len(references) == 0 {
		return nil
	}

	reg, err := ParseImportConfig(cmd.Flags.ImportCfg)
	if err != nil {
		return err
	}

	// If we're in a child build we must be re-use the same root build as our parent.
	root := cmd.WorkDir
	if val := os.Getenv(envVarOrchestrionRootBuild); val != "" {
		root = val
	}
	for importPath, kind := range references {
		if kind != typed.ImportStatement {
			continue
		}
		err := func() error {
			log.Printf("Building synthetically imported package %q if necessary...\n", importPath)
			outLog, _ := os.Create(path.Join(cmd.StageDir, fmt.Sprintf("build-%s.stdout.log", slugify(importPath))))
			defer outLog.Close()
			errLog, _ := os.Create(path.Join(cmd.StageDir, fmt.Sprintf("build-%s.stderr.log", slugify(importPath))))
			defer errLog.Close()

			dep, err := BuildPackage{
				ImportPath: importPath,
				RootBuild:  root,
				Stdout:     io.MultiWriter(os.Stdout, outLog),
				Stderr:     io.MultiWriter(os.Stderr, errLog),
			}.Run()

			if err != nil {
				return err
			}
			reg.Import(dep)
			return nil
		}()
		if err != nil {
			return err
		}
	}

	original := cmd.Flags.ImportCfg + dotOriginal
	log.Printf("Moving original importcfg file to %q...\n", original)
	if err := os.Rename(cmd.Flags.ImportCfg, original); err != nil {
		return fmt.Errorf("failed to move %q to %q: %w", cmd.Flags.ImportCfg, original, err)
	}

	log.Println("Overwriting importcfg with updated contents...")
	importcfg, err := os.Create(cmd.Flags.ImportCfg)
	if err != nil {
		return err
	}
	defer importcfg.Close()
	reg.WriteTo(importcfg)

	return nil
}

func (w *AspectWeaver) ProcessLink(cmd *proxy.LinkCommand) error {
	defer w.redirectLogger(path.Dir(path.Dir(cmd.Flags.Output)), "link")()
	log.Printf("Original command: %q\n", cmd.Args())

	linkReg, err := ParseImportConfig(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", cmd.Flags.ImportCfg, err)
	}

	var hadNew bool
	for importPath, object := range linkReg.PackageFile {
		var pkgReg *PackageRegister
		if strings.HasPrefix(object, gocache) {
			log.Printf("Package %q is from GOCACHE at %q\n", importPath, object)
			deps, err := listDependencies(object)
			if err != nil {
				return err
			}

			if len(deps) == 0 {
				continue
			}

			pkgReg = NewPackageRegister(importPath, "")
			for _, dep := range deps {
				if _, found := linkReg.PackageFile[dep]; found {
					continue
				}

				log.Printf("Adding resolution for new dependency %q...\n", dep)
				var archive string
				if link := LookupCompileUnit(dep, path.Dir(cmd.Flags.ImportCfg)); link != "" {
					archive = path.Join(link, "_pkg_.a")
					log.Printf("... resolved from index: %q\n", archive)
				} else {
					log.Println("... missing from index... resorting to 'go build'...")
					archive = path.Join(path.Dir(cmd.Flags.ImportCfg), fmt.Sprintf("%s.a", slugify(dep)))
					root := os.Getenv(envVarOrchestrionRootBuild)
					if root == "" {
						root = path.Dir(path.Dir(cmd.Flags.ImportCfg))
					}
					_, err := BuildPackage{
						ImportPath: dep,
						RootBuild:  root,
						ExtraArgs:  []string{"-o", archive},
					}.Run()
					if err != nil {
						return fmt.Errorf("failed to get package archive for %q: %w", dep, err)
					}
				}
				pkgReg.PackageFile[dep] = archive
			}
		} else {
			importcfg := path.Join(path.Dir(object), "importcfg")
			log.Printf("Loading %q dependencies from %q\n", importPath, importcfg)
			pkgReg, err = ParseImportConfig(importcfg)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					log.Printf("Could not find importcfg file for %q, ignoring...\n", object)
					continue
				}
				return err
			}
		}
		if linkReg.Combine(pkgReg, false) {
			log.Printf("Merged new entries for %q into package registry...\n", importPath)
			hadNew = true
		}

	}

	if !hadNew {
		log.Println("No alterations to importcfg.link necessary!")
		return nil
	}

	original := cmd.Flags.ImportCfg + dotOriginal
	log.Printf("Moving original importcfg.link file to %q...\n", original)
	if err := os.Rename(cmd.Flags.ImportCfg, original); err != nil {
		return fmt.Errorf("failed to move %q to %q: %w", cmd.Flags.ImportCfg, original, err)
	}

	log.Println("Overwriting importcfg.link with updated contents...")
	importcfg, err := os.Create(cmd.Flags.ImportCfg)
	if err != nil {
		return err
	}
	defer importcfg.Close()
	linkReg.WriteTo(importcfg)

	return nil
}

func (w *AspectWeaver) redirectLogger(stageDir string, cmd string) func() {
	logFile, err := os.Create(path.Join(stageDir, fmt.Sprintf("orchestrion.%d.aspectweaver.%s.log", os.Getpid(), cmd)))
	if err != nil {
		log.Printf("Failed to create AspectWeaver log file: %v\n", err)
		return func() {}
	}
	log.SetOutput(io.MultiWriter(logFile, os.Stderr))
	return func() {
		defer logFile.Close()
		log.SetOutput(os.Stderr)
	}
}

func linkCompileOutput(cmd *proxy.CompileCommand, linkname string) error {
	relOut, err := filepath.Rel(cmd.StageDir, cmd.Flags.Output)
	if err != nil {
		return fmt.Errorf("failed to compute relative path %q from %q: %w", cmd.Flags.Output, cmd.StageDir, err)
	}
	reused := path.Join(linkname, relOut)

	log.Printf("Copying output from %q to %q\n", cmd.Flags.Output, reused)
	if err := copyFile(reused, path.Join(cmd.Flags.Output)); err != nil {
		return fmt.Errorf("failed copying %q to %q: %w", reused, cmd.Flags.Output, err)
	}

	linked := path.Join(linkname, path.Base(cmd.Flags.ImportCfg))
	target := cmd.Flags.ImportCfg
	log.Printf("Moving original %q to %q\n", target, target+dotOriginal)
	if err := os.Rename(target, target+dotOriginal); err != nil {
		return fmt.Errorf("failed to move %q to %q: %w", target, target+dotOriginal, err)
	}
	log.Printf("Copying %q to %q\n", linked, target)
	if err := copyFile(linked, target); err != nil {
		return fmt.Errorf("failed copying %q to %q: %w", linked, target, err)
	}

	// Clean up superfluous .o files that the toolchain will ar into the `.a` file, causing duplicate symbol issues.
	entries, err := os.ReadDir(cmd.StageDir)
	if err != nil {
		return fmt.Errorf("failed to list directory %q: %w", cmd.StageDir, err)
	}
	var blankO []byte
	for _, entry := range entries {
		if path.Ext(entry.Name()) == ".o" {
			if blankO == nil {
				// Lazily create the blank object file...
				blankO, err = blankObjectFile(cmd.Flags.Package)
				if err != nil {
					return fmt.Errorf("failed creating blank object file: %w", err)
				}
			}

			filename := path.Join(cmd.StageDir, entry.Name())
			log.Printf("Replacing superfluous object file %q with blank\n", filename)
			if err := os.WriteFile(filename, blankO, 0o644); err != nil {
				return fmt.Errorf("error writing object file %q: %w", filename, err)
			}
		}
	}

	// Claim terminal success
	log.Println("Successfully re-used build output from another process, requesting command skip...")
	return proxy.ErrSkipCommand
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("failed to open %q for reading: %w", from, err)
	}

	dst, err := os.Create(to)
	if err != nil {
		return fmt.Errorf("failed to open %q for writing: %w", to, err)
	}

	_, err = io.Copy(dst, src)
	return err
}

// blankObjectFile creates a new blank object file suitable to be linked into a package with the
// provided import path.
func blankObjectFile(importPath string) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "blank*.o")
	if err != nil {
		return nil, fmt.Errorf("could not create temporay directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	blankS := path.Join(tmp, "blank.s")
	if err := os.WriteFile(blankS, []byte{}, 0o644); err != nil {
		return nil, fmt.Errorf("failed creating blank source file: %w", err)
	}

	blankO := path.Join(tmp, "blank.o")
	if err := exec.Command("go", "tool", "asm", "-p", importPath, "-o", blankO, blankS).Run(); err != nil {
		return nil, fmt.Errorf("failed to run 'go tool asm': %w", err)
	}

	return os.ReadFile(blankO)
}

var gocache string

func init() {
	var err error
	gocache, err = goproxy.Goenv("GOCACHE")
	if err != nil {
		panic(err)
	}
}
