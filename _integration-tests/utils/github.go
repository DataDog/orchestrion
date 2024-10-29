// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import "os"

// IsGithubActions is [true] if the execution is happening in Github Actions.
var IsGithubActions = os.Getenv("GITHUB_ACTIONS") == "true"
