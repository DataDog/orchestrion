// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"orchestrion/integration/validator/trace"
)

func main() {
	var (
		name           string
		validationFile string
		tracesFile     string
	)

	flag.StringVar(&name, "name", "", "The name of the test case")
	flag.StringVar(&validationFile, "validation", "", "The path to the validation.json file")
	flag.StringVar(&tracesFile, "traces", "", "The path to the traces.json file")
	flag.Parse()

	if name == "" {
		log.Println("Missing -name argument!")
		flag.Usage()
		os.Exit(2)
	}
	if validationFile == "" {
		log.Println("Missing -validation argument!")
		flag.Usage()
		os.Exit(2)
	}
	if tracesFile == "" {
		log.Println("Missing -trace argument!")
		flag.Usage()
		os.Exit(2)
	}

	var validation struct {
		Traces []*trace.Span `json:"output"`
	}
	if data, err := os.ReadFile(validationFile); err != nil {
		log.Fatalln("Error reading validation.json file:", err)
	} else if err := json.Unmarshal(data, &validation); err != nil {
		log.Fatalln("Error parsing contents of validation.json file:", err)
	}

	var traces []*trace.Span
	if data, err := os.ReadFile(tracesFile); err != nil {
		log.Fatalln("Error reading traces.json file:", err)
	} else if err := trace.ParseRaw(data, &traces); err != nil {
		log.Fatalln("Error parsing traces.json file:", err)
	}

	exitCode := 0
	for idx, reference := range validation.Traces {
		matches, diffs := reference.MatchesAny(traces)
		if matches {
			fmt.Printf("Successfully matched reference trace %d out of %d\n", idx+1, len(validation.Traces))
			continue
		}
		exitCode = 1
		fmt.Fprintf(os.Stderr, "Failed to match reference trace %d out of %d:\n%v\nDifferences:\n%s\n", idx+1, len(validation.Traces), reference, diffs)
	}

	os.Exit(exitCode)
}
