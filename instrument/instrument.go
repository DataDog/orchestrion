// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/instrument/event"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func Report(ctx context.Context, e event.Event, metadata ...any) context.Context {
	var span tracer.Span
	if e == event.EventStart || e == event.EventCall {
		var opts []tracer.StartSpanOption
		for i := 0; i < len(metadata); i += 2 {
			if i+1 >= len(metadata) {
				break
			}
			if k, ok := metadata[i].(string); ok {
				opts = append(opts, tracer.Tag(k, metadata[i+1]))
			}
		}
		_, ctx = tracer.StartSpanFromContext(ctx, getOpName(metadata...), opts...)
	} else if e == event.EventEnd || e == event.EventReturn {
		var ok bool
		span, ok = tracer.SpanFromContext(ctx)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "Error: Received end/return event but have no corresponding span in the context.\n")
			return ctx
		}
		span.Finish()
	}
	return ctx
}
