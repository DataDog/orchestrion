// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoFuncLookupsReturnNotFound(t *testing.T) {
	var fn function = noFunc{}

	for name, lookup := range map[string]func(string) (string, error){
		"ArgumentOfType":           fn.ArgumentOfType,
		"ArgumentThatImplements":   fn.ArgumentThatImplements,
		"ResultOfType":             fn.ResultOfType,
		"ResultThatImplements":     fn.ResultThatImplements,
		"LastResultThatImplements": fn.LastResultThatImplements,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := lookup("context.Context")
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	}

	t.Run("FinalResultImplements", func(t *testing.T) {
		got, err := fn.FinalResultImplements("error")
		require.NoError(t, err)
		assert.False(t, got)
	})
}

func TestNoFuncAccessorsError(t *testing.T) {
	var fn function = noFunc{}

	_, err := fn.Receiver()
	require.ErrorIs(t, err, errNoFunction)

	_, err = fn.Name()
	require.ErrorIs(t, err, errNoFunction)

	_, err = fn.Argument(0)
	require.ErrorIs(t, err, errNoFunction)

	_, err = fn.Result(0)
	require.ErrorIs(t, err, errNoFunction)
}
