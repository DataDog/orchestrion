// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"slices"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
)

// ImportsFilter filters out aspects that imply imports not present in the import map.
func importsFilter(aspects []*aspect.Aspect, importMap map[string]string, pkgImportPath string) []*aspect.Aspect {
	ctx := &may.PackageContext{
		ImportPath: pkgImportPath,
		ImportMap:  importMap,
	}
	return slices.DeleteFunc(aspects, func(a *aspect.Aspect) bool {
		if a.JoinPoint.PackageMayMatch(ctx) == may.CantMatch {
			return true
		}
		return false
	})
}

// contentContainsFilter filters out aspects AND files that imply content not present in the fileset.
// This works as follows:
// - For each file, we start reading it and storing its content in a []byte.
// - If we hit no limits, we check if the content contains the filter.
// - If all the strings from the (*Aspect).ImpliesContent() are present in the file, we keep the aspect for this file
// - If not, we remove the aspect from the list of aspects to run on this file
// - After all aspects are processed for a file, transform the []byte back to an io.ReadCloser and store it in the result map
// - If any limit was hit, we stop the filtering on this file and return it as is in the result map
func contentContainsFilter(aspects []*aspect.Aspect, fileContent []byte) []*aspect.Aspect {
	ctx := &may.FileMayMatchContext{
		FileContent: fileContent,
	}
	return slices.DeleteFunc(aspects, func(a *aspect.Aspect) (res bool) {
		if a.JoinPoint.FileMayMatch(ctx) == may.CantMatch {
			return true
		}
		return false
	})
}
