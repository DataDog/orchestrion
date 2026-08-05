// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func open(path string) {
	_, _ = os.OpenFile(path, os.O_RDONLY, 0)
}

func main() {
	const secret = "secret"
	cleanSource := secret
	cleanDirect := "clean"
	if cleanSource == secret {
		cleanDirect = "direct"
	} else {
		cleanDirect = "else"
	}
	open(cleanDirect)

	cleanLogical := "clean"
	if cleanSource[0] != 0 && cleanSource[0] == secret[0] {
		cleanLogical = "logical"
	}
	open(cleanLogical)

	cleanLoop := "clean"
	for len(cleanSource) > 0 {
		cleanLoop = "loop"
		break
	}
	open(cleanLoop)

	cleanSwitch := "clean"
	switch cleanSource[0] {
	case secret[0]:
		cleanSwitch = "switch"
	case 'x':
		cleanSwitch = "x"
	default:
		cleanSwitch = "default"
	}
	open(cleanSwitch)

	ready := make(chan struct{}, 1)
	ready <- struct{}{}
	cleanSelect := "clean"
	if cleanSource == secret {
		select {
		case <-ready:
			cleanSelect = "select"
		default:
			cleanSelect = "default"
		}
	}
	open(cleanSelect)

	if cleanSource == secret {
		open("inline")
	}

	source := os.Getenv("TAINT_PATH")
	direct := "clean"
	if source == secret {
		direct = "direct"
	} else {
		direct = "else"
	}
	open(direct)
	open("post-join-clean")

	logical := "clean"
	if source[0] != 0 && source[0] == secret[0] {
		logical = "logical"
	}
	open(logical)

	loop := "clean"
	for len(source) > 0 {
		loop = "loop"
		break
	}
	open(loop)

	switched := "clean"
	switch source[0] {
	case secret[0]:
		switched = "switch"
	case 'x':
		switched = "x"
	default:
		switched = "default"
	}
	open(switched)

	ready <- struct{}{}
	selected := "clean"
	if source == secret {
		select {
		case <-ready:
			selected = "select"
		default:
			selected = "default"
		}
	}
	open(selected)

	if source == secret {
		open("inline")
	}
}
