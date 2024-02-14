// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypeName(t *testing.T) {
	for name, err := range map[string]error{
		"0":                       errors.New(`invalid TypeName syntax: "0"`),
		"net/http.ResponseWriter": nil,
		"*net/http.Request":       nil,
	} {
		t.Run(name, func(t *testing.T) {
			_, e := NewTypeName(name)
			if err == nil {
				require.NoError(t, e)
			} else {
				require.EqualError(t, e, err.Error())
			}
		})

		t.Run(fmt.Sprintf("Must=%s", name), func(t *testing.T) {
			defer func() {
				e, _ := recover().(error)
				if err == nil {
					require.NoError(t, e)
				} else {
					require.EqualError(t, e, err.Error())
				}
			}()
			_ = MustTypeName(name)
		})
	}
}
