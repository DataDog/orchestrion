// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
)

func main() {
	f, err := os.OpenFile("LICENSE-3rdparty.csv", os.O_RDWR, os.ModePerm)
	if err != nil {
		log.Fatalf("cannot open csv: %s", err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("cannot to parse csv: %s", err)
	}
	offset := 1        // skip the first line (column names)
	pathRank := 1      // 2nd column
	copyrightRank := 3 // 4th column
	for i, rec := range records[offset:] {
		path := path.Join("/tmp/licenses", rec[pathRank])
		records[i+offset][copyrightRank] = scanPkg(path)
	}
	// reset the file pointer to rewrite from the begining
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		log.Fatal(err)
	}
	writer := csv.NewWriter(f)
	defer writer.Flush()
	if err := writer.WriteAll(records); err != nil {
		log.Fatalf("cannot to write csv: %s", err)
	}
}

func scanPkg(pkg string) string {
	entries, err := os.ReadDir(pkg)
	if err != nil {
		log.Printf("warn: skipping %s because of error %s", pkg, err)
		return "unknown"
	}
	var copyrights []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		c := scanFile(path.Join(pkg, entry.Name()))
		copyrights = append(copyrights, c...)
	}
	if len(copyrights) > 0 {
		return strings.Join(copyrights, " | ")
	}
	return "unknown"
}

var hasDigits = regexp.MustCompile(`\d`)

func isCopyright(line string) (string, bool) {
	line = strings.TrimSpace(line)
	return line, strings.HasPrefix(line, "Copyright") && hasDigits.MatchString(line)
}

func scanFile(fname string) []string {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("cannot open %s: %s", fname, err)
	}
	defer f.Close()
	var copyrights []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if l, ok := isCopyright(line); ok {
			copyrights = append(copyrights, l)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return copyrights
}
