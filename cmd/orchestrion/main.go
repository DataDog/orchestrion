package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/datadog/orchestrion"
)

func main() {
	var write bool
	flag.BoolVar(&write, "w", false, "if set, overwrite the current file with the instrumented file")
	flag.Parse()
	if len(flag.Args()) == 0 {
		return
	}
	process := func(fullName string, out io.Reader) {
		fmt.Printf("%s:\n", fullName)
		// write the output
		txt, _ := io.ReadAll(out)
		fmt.Println(string(txt))
	}
	if write {
		process = func(fullName string, out io.Reader) {
			fmt.Printf("overwriting %s:\n", fullName)
			// write the output
			txt, _ := io.ReadAll(out)
			err := os.WriteFile(fullName, txt, 0644)
			if err != nil {
				fmt.Printf("Writing file %s: %v\n", fullName, err)

			}
		}
	}
	for _, v := range flag.Args() {
		p, err := filepath.Abs(v)
		if err != nil {
			fmt.Printf("Sanitizing path (%s) failed: %v\n", v, err)
			continue
		}
		fmt.Printf("Scanning Package %s\n", p)
		err = orchestrion.ScanPackage(p, process)
		if err != nil {
			fmt.Printf("Failed to scan: %v\n", err)
		}
	}
}
