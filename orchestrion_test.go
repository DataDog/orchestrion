package orchestrion

import (
	"context"
	"io"
	"testing"
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
		if GetReportID(ctx) == "" {
			t.Errorf("Expected Report of StartEvent to generate a new ID.")
		}
	})

	t.Run("call", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventCall)
		if GetReportID(ctx) == "" {
			t.Errorf("Expected Report of CallEvent to generate a new ID.")
		}
	})

	t.Run("end", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventEnd)
		if GetReportID(ctx) != "" {
			t.Errorf("Expected Report of EndEvent not to generate a new ID.")
		}
	})

	t.Run("return", func(t *testing.T) {
		ctx := context.Background()
		ctx = Report(ctx, EventReturn)
		if GetReportID(ctx) != "" {
			t.Errorf("Expected Report of ReturnEvent not to generate a new ID.")
		}
	})
}
