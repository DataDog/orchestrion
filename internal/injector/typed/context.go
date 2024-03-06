// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import "context"

type key[T any] struct{}

func ContextWithValue[T any](ctx context.Context, value T) context.Context {
	return context.WithValue(ctx, key[T]{}, value)
}

func ContextValue[T any](ctx context.Context) (res T, ok bool) {
	val := ctx.Value(key[T]{})
	res, ok = val.(T)
	return
}
