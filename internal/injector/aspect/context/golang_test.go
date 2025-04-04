// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestSetAtLeast(t *testing.T) {
	var subject GoLangVersion

	require.Equal(t, GoLangVersion{}, subject)

	go1_0 := MustParseGoLangVersion("go1.0")
	go1_16 := MustParseGoLangVersion("go1.16")
	go1_18 := MustParseGoLangVersion("go1.18")

	// Upgrading from "anything goes" to "go1.0"...
	subject.SetAtLeast(go1_0)
	require.Equal(t, go1_0, subject)

	// Nothing changes (equal)...
	subject.SetAtLeast(go1_0)
	require.Equal(t, go1_0, subject)

	// Upgrading to go1.18...
	subject.SetAtLeast(go1_18)
	require.Equal(t, go1_18, subject)

	// Nothing happens, as "go1.16" is older than "go1.18"...
	subject.SetAtLeast(go1_16)
	require.Equal(t, go1_18, subject)
}

func TestString(t *testing.T) {
	require.Empty(t, GoLangVersion{}.String())
	require.Equal(t, "go1.18", MustParseGoLangVersion("go1.18").String())
}

func TestUnmarshalYAML(t *testing.T) {
	var parsed GoLangVersion

	require.Error(t, yaml.Unmarshal([]byte("{}"), &parsed))
	require.Equal(t, GoLangVersion{}, parsed)

	minor := rand.Int()
	if minor < 0 {
		minor = -minor
	}
	langStr := fmt.Sprintf("go1.%d", minor)
	lang := MustParseGoLangVersion(langStr)

	require.NoError(t, yaml.Unmarshal([]byte(langStr), &parsed))
	require.Equal(t, lang, parsed)

	require.NoError(t, yaml.Unmarshal([]byte("go0.1337"), &parsed))
	require.Equal(t, "go0.1337", parsed.String())

	require.Error(t, yaml.Unmarshal([]byte("go1.invalid"), &parsed), "invalid go language level")
}
