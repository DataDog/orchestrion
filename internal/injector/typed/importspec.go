// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"github.com/dave/dst"
)

// RetainedImportSpec returns the single import spec the injector keeps for a path when the file
// declares several for the same path. The dave/dst restorer supports exactly one name per import
// path per file (see dave/dst#45), so canonicalization reduces every path to this spec.
//
// The selection mirrors the historical preference order of canonicalizeImports: the first spec
// that carries a name wins, and a named spec is never displaced by a later blank one.
func RetainedImportSpec(specs []*dst.ImportSpec) *dst.ImportSpec {
	if len(specs) == 0 {
		return nil
	}
	retain := specs[0]
	for _, spec := range specs[1:] {
		if spec.Name == nil {
			continue
		}
		if retain.Name == nil {
			retain = spec
			continue
		}
		if retain.Name.Name == "_" && spec.Name.Name != "_" {
			retain = spec
			continue
		}
		// The first named spec wins; nothing can displace it.
		break
	}
	return retain
}

// SpecName returns the name the file binds an import spec to: the alias, or "" when the spec is
// unaliased.
func SpecName(spec *dst.ImportSpec) string {
	if spec == nil || spec.Name == nil {
		return ""
	}
	return spec.Name.Name
}
