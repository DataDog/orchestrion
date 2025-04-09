// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dave/jennifer/jen"
)

type flagSpec struct {
	Flag  string
	Value string
	Descr []string
}

func main() {
	var command string
	flag.Func("command", "The command to generate a parser for", func(val string) error {
		switch val {
		case "compile", "link":
			command = val
		default:
			return fmt.Errorf("unsupported command: %q", val)
		}
		return nil
	})
	flag.Parse()

	if command == "" {
		log.Fatalln("Missing value for required -command flag")
	}

	var (
		goFile    = requireEnv("GOFILE")
		goPackage = requireEnv("GOPACKAGE")
	)

	cmd := exec.Command("go", "tool", command, "-V=full")
	cmd.Env = append(cmd.Env, "LANG=C")
	var fullVersion bytes.Buffer
	cmd.Stdout = &fullVersion
	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}

	version := strings.TrimSpace(fullVersion.String())
	version = version[:strings.LastIndexByte(version, '.')]

	outFile := filepath.Join(goFile, "..", command+".flags.go")
	if content, err := os.ReadFile(outFile); err == nil {
		newMajorS, newMinorS, _ := strings.Cut(strings.Fields(version)[2][2:], ".")
		newMajor, _ := strconv.Atoi(newMajorS)
		newMinor, _ := strconv.Atoi(newMinorS)

		// versionTagRe matches the "<command> version goX.Y" tag that is expected to be present in generated files.
		// Example: https://regex101.com/r/MiYpBy/1
		versionTagRe := regexp.MustCompile(fmt.Sprintf(`(?m)"%s version go(\d+)\.(\d+)"`, command))
		if matches := versionTagRe.FindSubmatch(content); len(matches) > 0 {
			curMajor, _ := strconv.Atoi(string(matches[1]))
			curMinor, _ := strconv.Atoi(string(matches[2]))

			if curMajor > newMajor || (curMajor == newMajor && curMinor > newMinor) {
				if os.Getenv("CI") != "" {
					log.Fatalf("Generate must be run with go%d.%d or newer (was run with %d.%d)\n", curMajor, curMinor, newMajor, newMinor)
				}
				log.Printf("Skipping generation of %q, as it was generated against a more recent version of the go %s tool (%d.%d >= %d.%d)\n", outFile, command, curMajor, curMinor, newMajor, newMinor)
				return
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatalln(err)
	}

	cmd = exec.Command("go", "tool", command)
	cmd.Env = append(cmd.Env, "LANG=C")
	var buffer bytes.Buffer
	cmd.Stderr = &buffer
	_ = cmd.Run() // The command is expected to fail...

	reader := bufio.NewReader(&buffer)
	// Ensure the first line looks like usage instructions...
	line, _, err := reader.ReadLine()
	if err != nil {
		log.Fatalln(err)
	}
	if !bytes.HasPrefix(line, []byte("usage: ")) {
		log.Fatalf("Unexpected output from command:\n%s\n", string(line))
	}

	var (
		flags      []flagSpec
		knownFlags = make(map[string]struct{})
		// reFlag captures the flag and it's argument name (if present), as well as the documentation string that may
		// follow it. Example: https://regex101.com/r/vqsECV/1
		reFlag = regexp.MustCompile(`^  (-[^\s]+)(?: ([^\s]+))?(\t.+)?$`)
	)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		matches := reFlag.FindSubmatch(line)
		if len(matches) == 0 {
			text := strings.TrimSpace(string(line))
			if text != "" && len(flags) > 0 {
				flags[len(flags)-1].Descr = append(flags[len(flags)-1].Descr, text)
			}
			continue
		}

		spec := flagSpec{
			Flag:  string(matches[1]),
			Value: string(matches[2]),
		}
		if descr := strings.TrimSpace(string(matches[3])); descr != "" {
			spec.Descr = append(spec.Descr, descr)
		}
		flags = append(flags, spec)
		knownFlags[spec.Flag] = struct{}{}
	}

	if len(flags) == 0 {
		log.Fatalln("No flags found in command usage, this is unexpected!")
	}

	fset := token.NewFileSet()
	src, err := os.ReadFile(goFile)
	if err != nil {
		log.Fatalln(err)
	}
	parsed, err := parser.ParseFile(fset, goFile, src, 0)
	if err != nil {
		log.Fatalln(err)
	}

	typeName := command + "FlagSet"
	capturedFlags := make(map[string]string)
	for _, decl := range parsed.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.TYPE {
			continue
		}
		for _, spec := range decl.Specs {
			spec, ok := spec.(*ast.TypeSpec)
			if !ok || spec.Name.Name != typeName {
				continue
			}
			strct, ok := spec.Type.(*ast.StructType)
			if !ok {
				log.Fatalf("Expected a struct, got a %T\n", spec)
			}
			for _, field := range strct.Fields.List {
				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					log.Fatalln(err)
				}
				flag, err := strconv.Unquote(strings.TrimPrefix(tag, "ddflag:"))
				if err != nil {
					log.Fatalln(err)
				}
				if _, isKnown := knownFlags[flag]; !isKnown {
					log.Fatalf("Unknown captured flag: %q\n", flag)
				}
				capturedFlags[flag] = field.Names[0].Name
			}
		}
	}
	if len(capturedFlags) == 0 {
		log.Fatalf("Expected fields annotated with the `ddflag:\"-flag\"` tag in the %s struct\n", typeName)
	}

	file := jen.NewFile(goPackage)
	file.HeaderComment("// Unless explicitly stated otherwise all files in this repository are licensed")
	file.HeaderComment("// under the Apache License Version 2.0.")
	file.HeaderComment("// This product includes software developed at Datadog (https://www.datadoghq.com/).")
	file.HeaderComment("// Copyright 2023-present Datadog, Inc.\n")
	file.HeaderComment("// Code generated by 'go generate' DO NOT EDIT.\n")

	file.Func().
		Params(jen.Id("f").Op("*").Id(typeName)).
		Id("parse").
		Params(jen.Id("args").Index().String()).
		Params(
			jen.Index().String(),
			jen.Error(),
		).
		BlockFunc(func(g *jen.Group) {
			g.Id("flagSet").
				Op(":=").
				Qual("flag", "NewFlagSet").
				Call(
					jen.Lit(version),
					jen.Qual("flag", "ContinueOnError"),
				)

			// reDefault captures default values found in flag descriptions. See: https://regex101.com/r/jDEwWQ/1
			reDefault := regexp.MustCompile(`^(.+?)(?:\s+\(default\s+(.+?)\))?$`)
			for _, spec := range flags {
				fieldName, captured := capturedFlags[spec.Flag]

				var (
					funcName     string
					strDefault   string
					defaultValue *jen.Statement
					zeroValue    *jen.Statement
				)

				descr := strings.Join(spec.Descr, " ")
				if matches := reDefault.FindStringSubmatch(descr); matches[2] != "" {
					strDefault = matches[2]
					descr = matches[1]
				}

				switch spec.Value {
				case "":
					funcName = "Bool"
					zeroValue = jen.False()
					if strDefault == "" || strDefault == "false" {
						defaultValue = jen.False()
					} else {
						defaultValue = jen.True()
					}
				case "int":
					funcName = "Int"
					zeroValue = jen.Lit(0)
					if strDefault != "" {
						val, err := strconv.Atoi(strDefault)
						if err != nil {
							log.Fatalf("Invalid default value for an int flag: %s\n", strDefault)
						}
						defaultValue = jen.Lit(val)
					} else {
						defaultValue = jen.Lit(0)
					}
				default:
					funcName = "String"
					zeroValue = jen.Lit("")
					defaultValue = jen.Lit(strDefault)
				}

				flagName := jen.Lit(spec.Flag[1:])

				if funcName == "Bool" && spec.Flag == "-V" {
					var handler *jen.Statement
					if captured {
						handler = jen.Func().Params(jen.String()).Error().Block(
							jen.Id("f").Dot(fieldName).Op("=").True(),
							jen.Return(jen.Nil()),
						)
					} else {
						handler = jen.Func().Params(jen.String()).Error().Block(jen.Return(jen.Nil()))
					}
					g.Id("flagSet").Dot("BoolFunc").Call(flagName, jen.Lit(descr), handler)
				} else if captured {
					g.Id("flagSet").Dot(funcName+"Var").Call(jen.Op("&").Id("f").Dot(fieldName), flagName, defaultValue, jen.Lit(descr))
				} else {
					g.Id("flagSet").Dot(funcName).Call(flagName, zeroValue, jen.Lit(descr))
				}
			}
			g.Line()

			g.Id("err").Op(":=").Id("flagSet").Dot("Parse").Call(jen.Id("args"))
			g.Return(
				jen.Id("flagSet").Dot("Args").Call(),
				jen.Id("err"),
			)
		})

	out, err := os.Create(outFile)
	if err != nil {
		log.Fatalln(err)
	}
	defer out.Close()

	if err := file.Render(out); err != nil {
		log.Fatalln(err)
	}
}

func (f *flagSpec) String() string {
	if f.Value != "" {
		return fmt.Sprintf("%s %s", f.Flag, f.Value)
	}
	return f.Flag
}

func requireEnv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Fatalf("Missing environment variable: $%s\n", name)
	}
	return val
}
