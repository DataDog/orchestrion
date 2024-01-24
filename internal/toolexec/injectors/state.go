package injectors

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
)

var ddStateFilePath = fmt.Sprintf("%s/__dd_build.state", os.TempDir())

// State represents the state of compilation of an app
// It is used to keep track of whatever packages get built
// This is saved to the disk in between toolexec calls in order
// to keep some state from one call to another (mainly compile -> link)
type State struct {
	// mapping import : dependencies
	Deps map[string]PkgRegister
}

func (s *State) serialize() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(*s); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *State) SaveToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := s.serialize()

	if err == nil {
		file.Write(data)
	}

	return err
}

func StateFromFile(path string) (State, error) {
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
