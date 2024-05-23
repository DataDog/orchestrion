// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import "github.com/dave/jennifer/jen"

type AsCode interface {
	// AsCode produces a jen.Code representation of the receiver.
	AsCode() jen.Code
}

type ImportAdder interface {
	// AddedImports returns the list of import paths the receiver may introduce in
	// modified code.
	AddedImports() []string
}
