// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInfoVersion(t *testing.T) {
	assert.True(t, buildInfoIsDev)
	assert.Equal(t, tag+"+devel", buildInfoVersion)
}
