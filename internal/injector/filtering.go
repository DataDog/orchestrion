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

// packageFilterAspects filters out aspects that imply imports not present in the import map.
func (i *Injector) packageFilterAspects(aspects []*aspect.Aspect) []*aspect.Aspect {
	ctx := &may.PackageContext{
		ImportPath: i.ImportPath,
		ImportMap:  i.ImportMap,
		TestMain:   i.TestMain,
	}
	return slices.DeleteFunc(aspects, func(a *aspect.Aspect) bool {
		return a.JoinPoint.PackageMayMatch(ctx) == may.CantMatch
	})
}

// fileFilterAspects filters out aspects for a specific file.
func fileFilterAspects(aspects []*aspect.Aspect, fileContent []byte, packageName string) []*aspect.Aspect {
	aspectsCopy := make([]*aspect.Aspect, len(aspects))
	for i, a := range aspects {
		aspectsCopy[i] = a
	}

	ctx := &may.FileContext{
		FileContent: fileContent,
		PackageName: packageName,
	}

	return slices.DeleteFunc(aspectsCopy, func(a *aspect.Aspect) (res bool) {
		return a.JoinPoint.FileMayMatch(ctx) == may.CantMatch
	})
}
