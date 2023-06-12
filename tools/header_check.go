package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	expected := []string{
		"// Unless explicitly stated otherwise all files in this repository are licensed",
		"// under the Apache License Version 2.0.",
		"// This product includes software developed at Datadog (https://www.datadoghq.com/).",
		"// Copyright 2023-present Datadog, Inc.",
	}
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Checking copyright headers of Go files in %q recursively", pwd)
	var errors []string
	err = filepath.Walk(pwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			log.Fatalf("Error scanning %q: %s", info.Name(), err)
		}
		defer file.Close()
		r := bufio.NewReader(file)
		for i := 0; i < 4; i++ {
			line, _, err := r.ReadLine()
			if err != nil {
				log.Fatal(path, i, err)
			}
			if expected[i] != string(line) {
				errors = append(errors, fmt.Sprintf("File %s does not contain copyright headers!", info.Name()))
				break
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}
