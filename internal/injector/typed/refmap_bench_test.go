// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dave/dst"
)

// benchSource builds a single function with size references to strings interleaved with
// size filler declarations in one block: the worst shape for scope lookups during alias
// planning, since every reference must be checked against the declarations of its block.
func benchSource(size int) string {
	var b strings.Builder
	b.WriteString("package test\n\nimport (\n  \"os\"\n  str \"strings\"\n)\n\nfunc helper(s string) string { return str.ToLower(s) }\n\nfunc join() string {\n")
	for i := range size {
		fmt.Fprintf(&b, "  _ = str.ToLower(\"x\")\n")
		fmt.Fprintf(&b, "  x%d := %d\n  _ = x%d\n", i, i, i)
	}
	b.WriteString("  return os.Getenv(\"HOME\")\n}\n")
	return b.String()
}

// BenchmarkAliasPlanning measures one full alias-planning pass (AddSyntheticImports) over a
// file with the given number of reference/declaration pairs in a single block. The scope
// lookup must stay linear, or at most n log n, in the size of the file: doubling the size
// must not more than double the cost per statement.
func BenchmarkAliasPlanning(b *testing.B) {
	for _, size := range []int{256, 512, 1024} {
		b.Run(fmt.Sprintf("pairs-%d", size), func(b *testing.B) {
			envs := decoratePackage(b, []srcFile{src(benchSource(size))})
			env := envs[0]
			chain := env.nodeChain(env.joinCall(b, 0))

			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				// A fresh copy per iteration, so every pass plans the original state.
				file := dst.Clone(env.file).(*dst.File)
				refMap := new(ReferenceMap)
				*refMap = NewReferenceMap(env.nodeMap, env.scopes)
				refMap.AddImport(file, chain, "strings", "strings")
				b.StartTimer()
				refMap.AddSyntheticImports(file)
			}
		})
	}
}
