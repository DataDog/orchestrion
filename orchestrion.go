package orchestrion

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
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

func InsertHeader(r *http.Request) {
	ctx := r.Context()
	reportID := GetReportID(ctx)
	if reportID != "" {
		r.Header.Set("x-trace-id", reportID)
	}
}

func HandleHeader(r *http.Request) *http.Request {
	reportID := r.Header.Get("x-trace-id")
	if reportID == "" {
		return r
	}
	ctx := r.Context()
	if GetReportID(ctx) == "" {
		ctx = AddReportID(ctx, reportID)
		r = r.WithContext(ctx)
	}
	return r
}

type reportIDType int

const (
	_ reportIDType = iota
	reportKey
)

func AddReportID(ctx context.Context, reportID string) context.Context {
	return context.WithValue(ctx, reportKey, reportID)
}

func GetReportID(ctx context.Context) string {
	val, ok := ctx.Value(reportKey).(string)
	if !ok {
		return ""
	}
	return val
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

func Report(ctx context.Context, e Event, metadata ...any) context.Context {
	var reportID string
	if e == EventStart || e == EventCall {
		reportID = uuid.NewString()
		ctx = AddReportID(ctx, reportID)
	} else {
		reportID = GetReportID(ctx)
	}

	frames := StackTrace(buildStackTrace())
	frame, _ := frames.Next()
	file := ""
	line := 0
	funcName := ""
	if frame.Func != nil {
		file, line = frame.Func.FileLine(frame.PC)
		funcName = frame.Func.Name()
	}

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

	var s strings.Builder
	s.WriteString(fmt.Sprintf(`{"time":"%s", "reportID":"%s", "event":"%s"`,
		time.Now(), reportID, e))
	s.WriteString(fmt.Sprintf(`, "function":"%s", "file":"%s", "line":%d`, funcName, file, line))
	if len(metadata)%2 != 0 {
		metadata = append(metadata, "")
	}
	for i := 0; i < len(metadata); i += 2 {
		s.WriteString(fmt.Sprintf(`, "%s":"%s"`, metadata[i], metadata[i+1]))
	}
	s.WriteString("}")
	fmt.Println(s.String())
	return ctx
}

func WrapHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = HandleHeader(r)
		r = r.WithContext(Report(r.Context(), EventStart, "name", "FooHandler", "verb", r.Method))
		defer Report(r.Context(), EventEnd, "name", "FooHandler", "verb", r.Method)
		handler.ServeHTTP(w, r)
	})
}

func WrapHandlerFunc(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r = HandleHeader(r)
		r = r.WithContext(Report(r.Context(), EventStart, "name", "FooHandler", "verb", r.Method))
		defer Report(r.Context(), EventEnd, "name", "FooHandler", "verb", r.Method)
		handlerFunc(w, r)
	}

}
