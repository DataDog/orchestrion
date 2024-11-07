// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument_test

import (
	"context"
	"testing"

	"github.com/DataDog/orchestrion/instrument"
	"github.com/DataDog/orchestrion/instrument/event"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func TestReport(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		ctx := context.Background()
		ctx = instrument.Report(ctx, event.EventStart)
		if _, ok := tracer.SpanFromContext(ctx); !ok {
			t.Errorf("Expected Report of StartEvent to generate a new ID.")
		}
	})

	t.Run("call", func(t *testing.T) {
		ctx := context.Background()
		ctx = instrument.Report(ctx, event.EventCall)
		if _, ok := tracer.SpanFromContext(ctx); !ok {
			t.Errorf("Expected Report of CallEvent to generate a new ID.")
		}
	})

	t.Run("end", func(t *testing.T) {
		ctx := context.Background()
		ctx = instrument.Report(ctx, event.EventEnd)
		if _, ok := tracer.SpanFromContext(ctx); ok {
			t.Errorf("Expected Report of EndEvent not to generate a new ID.")
		}
	})

	t.Run("return", func(t *testing.T) {
		ctx := context.Background()
		ctx = instrument.Report(ctx, event.EventReturn)
		if _, ok := tracer.SpanFromContext(ctx); ok {
			t.Errorf("Expected Report of ReturnEvent not to generate a new ID.")
		}
	})
}
