// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"fmt"
	"go/version"

	"gopkg.in/yaml.v3"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

// GoLangVersion represents a go language level. It's a string of the form "go1.18".
type GoLangVersion struct {
	label string
}

func ParseGoLangVersion(lang string) (GoLangVersion, error) {
	if !version.IsValid(lang) {
		return GoLangVersion{}, fmt.Errorf(`invalid go language level (expected e.g, "go1.18"): %q`, lang)
	}
	return GoLangVersion{lang}, nil
}

func MustParseGoLangVersion(lang string) GoLangVersion {
	val, err := ParseGoLangVersion(lang)
	if err != nil {
		panic(err)
	}
	return val
}

func (g GoLangVersion) String() string {
	return g.label
}

// IsAny returns true if the GoLang version selection is blank, meaning no particular constraint is
// imposed on language level.
func (g GoLangVersion) IsAny() bool {
	return g.label == ""
}

func (g *GoLangVersion) SetAtLeast(other GoLangVersion) {
	if Compare(*g, other) >= 0 {
		return
	}
	g.label = other.label
}

func Compare(left GoLangVersion, right GoLangVersion) int {
	return version.Compare(version.Lang(left.label), version.Lang(right.label))
}

var _ fingerprint.Hashable = (*GoLangVersion)(nil)

func (g GoLangVersion) Hash(h *fingerprint.Hasher) error {
	return h.Named("GoLangVersion", fingerprint.String(g.label))
}

var _ yaml.Unmarshaler = (*GoLangVersion)(nil)

func (g *GoLangVersion) UnmarshalYAML(node *yaml.Node) error {
	var lang string
	if err := node.Decode(&lang); err != nil {
		return err
	}

	val, err := ParseGoLangVersion(lang)
	if err != nil {
		return err
	}

	*g = val

	return nil
}
