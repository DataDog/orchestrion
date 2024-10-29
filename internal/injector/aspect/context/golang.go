// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"fmt"
	"go/version"

	"gopkg.in/yaml.v3"
)

// GoLang represents a go language level. It's a string of the form "go1.18".
type GoLang struct {
	label string
}

func ParseGoLang(lang string) (GoLang, error) {
	if !version.IsValid(lang) {
		return GoLang{}, fmt.Errorf(`invalid go language level (expected e.g, "go1.18"): %q`, lang)
	}
	return GoLang{lang}, nil
}

func MustParseGoLang(lang string) GoLang {
	val, err := ParseGoLang(lang)
	if err != nil {
		panic(err)
	}
	return val
}

func (g GoLang) String() string {
	return g.label
}

// IsAny returns true if the GoLang version selection is blank, meaning no particular constraint is
// imposed on language level.
func (g GoLang) IsAny() bool {
	return g.label == ""
}

func (g *GoLang) SetAtLeast(other GoLang) {
	if Compare(*g, other) >= 0 {
		return
	}
	g.label = other.label
}

func Compare(left GoLang, right GoLang) int {
	return version.Compare(version.Lang(left.label), version.Lang(right.label))
}

var _ yaml.Unmarshaler = (*GoLang)(nil)

func (g *GoLang) UnmarshalYAML(node *yaml.Node) error {
	var lang string
	if err := node.Decode(&lang); err != nil {
		return err
	}

	val, err := ParseGoLang(lang)
	if err != nil {
		return err
	}

	*g = val

	return nil
}
