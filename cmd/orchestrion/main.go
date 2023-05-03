package main

import (
	"flag"
	"fmt"
	"io"
	"os"

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
				fmt.Println(err)
			}
		}
	}
	for _, v := range flag.Args() {
		err := orchestrion.ScanPackage(v, process)
		if err != nil {
			fmt.Println(err)
		}
	}
}
