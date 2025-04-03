// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package version

import "runtime/debug"

const (
	// tag specifies the current release tag. It needs to be manually updated.
	tag       = "v1.3.1"
	devSuffix = "+devel"
)

var (
	buildInfoVersion string
	buildInfoIsDev   bool
)

// Tag returns the version tag for this orchestrion build.
func Tag() string {
	if buildInfoVersion != "" {
		return buildInfoVersion
	}
	return tag
}

// TagInfo returns the static tag and a boolean determining whether this is a
// development build.
func TagInfo() (staticTag string, isDev bool) {
	return tag, buildInfoIsDev
}

func init() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	version := bi.Main.Version
	if bi.Main.Replace != nil {
		version = bi.Main.Replace.Version
	}

	switch version {
	case "", "(devel)":
		// In tests, the [debug.BuildInfo.Main] has an empty version; and in builds
		// of the command, it has a "(devel)" version.
		buildInfoVersion = tag + devSuffix
		buildInfoIsDev = true
	default:
		buildInfoVersion = bi.Main.Version
	}
}
