package orchestrion

import (
	"context"
	"io"
	"testing"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func TestScanPackageDST(t *testing.T) {
	process := func(fullName string, out io.Reader) {
		io.ReadAll(out)
	}
	ScanPackage("./cmd/samples", process)
}

func TestReport(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventStart)
		if _, ok := tracer.SpanFromContext(ctx); !ok {
			t.Errorf("Expected Report of StartEvent to generate a new ID.")
		}
	})

	t.Run("call", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventCall)
		if _, ok := tracer.SpanFromContext(ctx); !ok {
			t.Errorf("Expected Report of CallEvent to generate a new ID.")
		}
	})

	t.Run("end", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventEnd)
		if _, ok := tracer.SpanFromContext(ctx); ok {
			t.Errorf("Expected Report of EndEvent not to generate a new ID.")
		}
	})

	t.Run("return", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventReturn)
		if _, ok := tracer.SpanFromContext(ctx); ok {
			t.Errorf("Expected Report of ReturnEvent not to generate a new ID.")
		}
	})
}

func TestGetOpName(t *testing.T) {
	for _, tt := range []struct {
		metadata []any
		opname   string
	}{
		{
			metadata: []any{"foo", "bar", "verb", "just-verb"},
			opname:   "just-verb",
		},
		{
			metadata: []any{"foo", "bar", "function-name", "just-function-name"},
			opname:   "just-function-name",
		},
		{
			metadata: []any{"foo", "bar", "verb", "verb-function-name", "function-name", "THIS IS WRONG"},
			opname:   "verb-function-name",
		},
		{
			// Checking different order
			metadata: []any{"foo", "bar", "function-name", "THIS IS WRONG", "verb", "verb-function-name"},
			opname:   "verb-function-name",
		},
	} {
		t.Run(tt.opname, func(t *testing.T) {
			n := getOpName(tt.metadata...)
			if n != tt.opname {
				t.Errorf("Expected %s, but got %s\n", tt.opname, n)
			}
		})
	}
}
