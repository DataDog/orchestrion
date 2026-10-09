// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRequest struct {
	ID int `json:"id"`
}

func (testRequest) ResponseIs(bool)                  {}
func (testRequest) ForeachSpanTag(func(string, any)) {}

func TestHandleRequestContexts(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		spans = make(map[int]bool) // Whether the context of each request had a span
	)
	handle := common.HandleRequest(context.Background(), func(ctx context.Context, req testRequest) (bool, error) {
		defer wg.Done()
		_, found := tracer.SpanFromContext(ctx)
		mu.Lock()
		defer mu.Unlock()
		spans[req.ID] = found
		return true, nil
	})
	message := func(id int, parent *tracer.Span) *nats.Msg {
		msg := &nats.Msg{Subject: "test", Data: fmt.Appendf(nil, `{"id":%d}`, id), Header: nats.Header{}}
		if parent != nil {
			require.NoError(t, tracer.Inject(parent.Context(), traceutil.NATSCarrier{Msg: msg}))
		}
		return msg
	}

	// Traced requests are handled concurrently, each with its own span.
	const traced = 8
	wg.Add(traced)
	for id := range traced {
		parent := tracer.StartSpan("client")
		handle(message(id, parent))
		parent.Finish()
	}
	wg.Wait()
	for id := range traced {
		assert.True(t, spans[id], "traced request %d has no span", id)
	}

	// The spans of previous requests do not leak into the context of the following ones.
	wg.Add(1)
	handle(message(traced, nil))
	wg.Wait()
	assert.False(t, spans[traced], "untraced request has a span")
}
