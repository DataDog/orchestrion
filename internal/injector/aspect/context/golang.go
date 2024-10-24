// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// GoLang represents a go language level. It's a string of the form "go1.18".
type GoLang struct {
	semver string
}

func ParseGoLang(lang string) (GoLang, error) {
	// Validate the language level value...
	if !strings.HasPrefix(lang, "go1.") {
		return GoLang{}, fmt.Errorf(`invalid go language level (expected e.g, "go1.18"): %q`, lang)
	}
	for _, r := range lang[4:] {
		if !unicode.IsDigit(r) {
			return GoLang{}, fmt.Errorf(`invalid go language level (expected e.g, "go1.18"): %q`, lang)
		}
	}
	// The semver package requires version strings have the "v" prefix...
	return GoLang{"v" + lang[2:]}, nil
}

func MustParseGoLang(lang string) GoLang {
	val, err := ParseGoLang(lang)
	if err != nil {
		panic(err)
	}
	return val
}

func (g GoLang) String() string {
	if g.IsAny() {
		return ""
	}

	// We print this out iwth a "go" prefix instead of the semver "v" prefix.
	return "go" + g.semver[1:]
}

// IsAny returns true if the GoLang version selection is blank, meaning no particular constraint is
// imposed on language level.
func (g GoLang) IsAny() bool {
	return g.semver == ""
}

func (g *GoLang) SetAtLeast(other GoLang) {
	if Compare(*g, other) >= 0 {
		return
	}
	g.semver = other.semver
}

func Compare(left GoLang, right GoLang) int {
	if left == right {
		return 0
	}

	if left.IsAny() {
		// Blank is lower than any other version...
		return -1
	}

	return semver.Compare(left.semver, right.semver)
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
