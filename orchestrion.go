// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package orchestrion

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"google.golang.org/grpc"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// if a function meets the handlerfunc type, insert code to:
// get the header from the request and look for the trace id
// if it's there but not in the context, add it to the context, add the context back to the request
// if it's not there and there's no traceid in the context, generate a guid, add it to the context, put the context back into the request
// output an "event" with a start message that has the method name, verb, id
// add a defer that outputs an event with an end message that has method name, verb, id
// can do this by having a function call that takes in the request and returns a request
/*
convert this:
func doThing(w http.ResponseWriter, r *http.Request) {
	// stuff here
}

to this:
func doThing(w http.ResponseWriter, r *http.Request) {
	//dd:startinstrument
	r = HandleHeader(r)
	Report(r.Context(), EventStart, "name", "doThing", "verb", r.Method)
	defer Report(r.Context(), EventEnd, "name", "doThing", "verb", r.Method)
	//dd:endinstrument
	// stuff here
}

Will need to properly capture the name of r from the function signature


For a client:
If you see a NewRequestWithContext or NewRequest call:
after the call,
- see if there's a traceid in the context
- if not add one and make a new context and request
- insert the header with the traceid

convert this:
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "localhost:8080", strings.NewReader(os.Args[1]))
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)

to this:
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "localhost:8080", strings.NewReader(os.Args[1]))
	//dd:startinstrument
	if req != nil {
		req = InsertHeader(req)
		Report(req.Context(), EventCall, "url", req.URL, "method", req.Method)
		defer Report(req.Context(), EventReturn, "url", req.URL, "method", req.Method)
	}
	//dd:endinstrument
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)

Will need to properly capture the name of req from the return values of the NewRequest/NewRequestWithContext call

Once we have this working for these simple cases, can work on harder ones!
*/

func InsertHeader(r *http.Request) *http.Request {
	span, ok := tracer.SpanFromContext(r.Context())
	if !ok {
		return r
	}
	r = r.Clone(r.Context())
	tracer.Inject(span.Context(), tracer.HTTPHeadersCarrier(r.Header))
	return r
}

//go:generate stringer -type=Event
type Event int

const (
	_ Event = iota
	EventStart
	EventEnd
	EventCall
	EventReturn
	EventDBCall
	EventDBReturn
)

func buildStackTrace() []uintptr {
	pc := make([]uintptr, 2)
	n := runtime.Callers(3, pc)
	pc = pc[:n]
	return pc
}

func StackTrace(trace []uintptr) *runtime.Frames {
	return runtime.CallersFrames(trace)
}

func getOpName(metadata ...any) string {
	rank := map[string]int{
		"verb":          1,
		"function-name": 2,
	}

	var (
		opname string
		oprank int = 10_000 // just a higher number than any key in the rank map.
	)
	for i := 0; i < len(metadata); i += 2 {
		if i+1 >= len(metadata) {
			break
		}
		if k, ok := metadata[i].(string); ok {
			if r, ok := rank[k]; ok && r < oprank {
				if on, ok := metadata[i+1].(string); ok {
					opname = on
					oprank = r
					continue
				}
			}
		}
	}
	return opname
}

func Report(ctx context.Context, e Event, metadata ...any) context.Context {
	var span tracer.Span
	if e == EventStart || e == EventCall {
		var opts []tracer.StartSpanOption
		for i := 0; i < len(metadata); i += 2 {
			if i+1 >= len(metadata) {
				break
			}
			if k, ok := metadata[i].(string); ok {
				opts = append(opts, tracer.Tag(k, metadata[i+1]))
			}
		}
		span, ctx = tracer.StartSpanFromContext(ctx, getOpName(metadata...), opts...)
	} else if e == EventEnd || e == EventReturn {
		var ok bool
		span, ok = tracer.SpanFromContext(ctx)
		if !ok {
			fmt.Printf("Error: Received end/return event but have no corresponding span in the context.\n")
			return ctx
		}
		span.Finish()
	}

	// 	frames := StackTrace(buildStackTrace())
	// 	frame, _ := frames.Next()
	// 	file := ""
	// 	line := 0
	// 	funcName := ""
	// 	if frame.Func != nil {
	// 		file, line = frame.Func.FileLine(frame.PC)
	// 		funcName = frame.Func.Name()
	// 	}

	// in case we end up needing to walk further up, here's code to do that
	//for {
	//	frame, more := frames.Next()
	//	if frame.Func != nil {
	//		file, line := frame.Func.FileLine(frame.PC)
	//		fmt.Printf("Function %s in file %s on line %d\n", frame.Func.Name(),
	//			file, line)
	//	}
	//	if !more {
	//		break
	//	}
	//}

	// 	var s strings.Builder
	// 	s.WriteString(fmt.Sprintf(`{"time":"%s", "reportID":"%s", "event":"%s"`,
	// 		time.Now(), reportID, e))
	// 	s.WriteString(fmt.Sprintf(`, "function":"%s", "file":"%s", "line":%d`, funcName, file, line))
	// 	if len(metadata)%2 != 0 {
	// 		metadata = append(metadata, "")
	// 	}
	// 	for i := 0; i < len(metadata); i += 2 {
	// 		s.WriteString(fmt.Sprintf(`, "%s":"%s"`, metadata[i], metadata[i+1]))
	// 	}
	// 	s.WriteString("}")
	// 	fmt.Println(s.String())

	fmt.Printf("%v: %v\n", e, span)
	return ctx
}

func WrapHandler(handler http.Handler) http.Handler {
	return httptrace.WrapHandler(handler, "", "")
	// TODO: We'll reintroduce this later when we stop hard-coding dd-trace-go as above.
	//	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//		r = HandleHeader(r)
	//		r = r.WithContext(Report(r.Context(), EventStart, "name", "FooHandler", "verb", r.Method))
	//		defer Report(r.Context(), EventEnd, "name", "FooHandler", "verb", r.Method)
	//		handler.ServeHTTP(w, r)
	//	})
}

func WrapHandlerFunc(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httptrace.TraceAndServe(handlerFunc, w, r, &httptrace.ServeConfig{})
	})
	// TODO: We'll reintroduce this later when we stop hard-coding dd-trace-go as above.
	//	return func(w http.ResponseWriter, r *http.Request) {
	//		r = HandleHeader(r)
	//		r = r.WithContext(Report(r.Context(), EventStart, "name", "FooHandler", "verb", r.Method))
	//		defer Report(r.Context(), EventEnd, "name", "FooHandler", "verb", r.Method)
	//		handlerFunc(w, r)
	//	}
}

func WrapHTTPClient(client *http.Client) *http.Client {
	// TODO: Stop hard-coding dd-trace-go.
	return httptrace.WrapClient(client)
}

func GRPCStreamServerInterceptor() grpc.ServerOption {
	return grpc.StreamInterceptor(grpctrace.StreamServerInterceptor())
}

func GRPCUnaryServerInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(grpctrace.UnaryServerInterceptor())
}

func Init() func() {
	tracer.Start()
	return tracer.Stop
}
