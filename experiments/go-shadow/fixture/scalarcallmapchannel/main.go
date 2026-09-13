// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func byteIdentity(value byte) byte {
	return value
}

//go:noinline
func runeIdentity(value rune) rune {
	return value
}

func main() {
	source := os.Getenv("TAINT_PATH")
	dirtyByte := source[0]
	dirtyRune := rune(dirtyByte)

	_, _ = os.Open(string(byteIdentity(dirtyByte)))
	_, _ = os.Open(string(runeIdentity(dirtyRune)))

	byteValues := map[string]byte{"dirty": dirtyByte}
	_, _ = os.Open(string(byteValues["dirty"]))
	byteValues["dirty"] = 'x'
	_, _ = os.Open(string(byteValues["dirty"]))

	runeValues := map[string]rune{"dirty": dirtyRune}
	_, _ = os.Open(string(runeValues["dirty"]))
	runeValues["dirty"] = 'x'
	_, _ = os.Open(string(runeValues["dirty"]))

	byteChannel := make(chan byte, 1)
	byteChannel <- dirtyByte
	_, _ = os.Open(string(<-byteChannel))
	byteChannel <- 'x'
	_, _ = os.Open(string(<-byteChannel))

	runeChannel := make(chan rune, 1)
	runeChannel <- dirtyRune
	_, _ = os.Open(string(<-runeChannel))
	runeChannel <- 'x'
	_, _ = os.Open(string(<-runeChannel))
}
