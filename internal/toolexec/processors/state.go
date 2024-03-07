// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"encoding/gob"
	"os"
	"path"
)

var ddStateFilePath = path.Join(os.TempDir(), ".dd_build.state")

// State represents the state of compilation of an app
// It is used to keep track of whatever packages get built
// This is saved to the disk in between toolexec calls in order
// to keep some state from one call to another (mainly compile -> link)
type State struct {
	// mapping import : dependencies
	Deps map[string]PackageRegister
}

// SaveToFile serializes s and writes the output to a file
func (s *State) SaveToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	return enc.Encode(*s)
}

// LoadFromFile reads the file at path and deserializes its content into a State object
func LoadFromFile(path string) (State, error) {
	var s State

	file, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	if err := dec.Decode(&s); err != nil {
		return s, err
	}

	return s, nil
}
