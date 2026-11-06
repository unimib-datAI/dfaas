// Entry point of the OpenWhisk Go action. The runtime calls Main(args) with the
// Web Action parameters and expects a map with statusCode, headers and body as
// response. There is no func main because the runtime provides it.
//
// See: https://github.com/apache/openwhisk/blob/master/docs/actions-go.md
// See: https://github.com/apache/openwhisk/blob/master/docs/webactions.md
package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	defaultMiB     = 128
	defaultSeconds = 5
	maxMiB         = 1024
	maxSeconds     = 5 * 60
)

// response builds a plain text Web Action response.
func response(statusCode int, body string) map[string]interface{} {
	return map[string]interface{}{
		"statusCode": statusCode,
		"headers":    map[string]interface{}{"Content-Type": "text/plain; charset=utf-8"},
		"body":       body,
	}
}

// intParam reads an integer parameter. If the parameter is absent, def is
// returned. If it is present but not an integer in [min, max], an error is
// returned.
func intParam(args map[string]interface{}, name string, def, min, max int) (int, error) {
	value, ok := args[name]
	if !ok {
		return def, nil
	}

	// Query parameters are strings, but parameters bound to the action or given
	// as JSON can be numbers.
	raw := fmt.Sprint(value)
	if raw == "" {
		return def, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return 0, fmt.Errorf("invalid %q=%q: must be an integer between %d and %d", name, raw, min, max)
	}
	return v, nil
}

func Main(args map[string]interface{}) map[string]interface{} {
	// OpenWhisk gives the HTTP method in lowercase.
	if args["__ow_method"] != "get" {
		return response(405, "Method not allowed\n")
	}

	mb, err := intParam(args, "mb", defaultMiB, 1, maxMiB)
	if err != nil {
		return response(400, err.Error()+"\n")
	}

	seconds, err := intParam(args, "seconds", defaultSeconds, 0, maxSeconds)
	if err != nil {
		return response(400, err.Error()+"\n")
	}

	buf := make([]byte, mb*1024*1024)

	// Touch each page to make sure all memory is used. In this way we force the
	// OS to allocate all pages without touching all bytes of the page.
	pageSize := os.Getpagesize()
	for i := 0; i < len(buf); i += pageSize {
		buf[i] = 1
	}
	buf[len(buf)-1] = 1

	time.Sleep(time.Duration(seconds) * time.Second)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// Keep the buffer reachable until after the measurement.
	runtime.KeepAlive(buf)

	heapMiB := mem.HeapAlloc / (1024 * 1024)
	message := fmt.Sprintf("requested_mib=%d duration_seconds=%d heap_alloc_mib=%d", mb, seconds, heapMiB)

	log.Printf("memory_stress_completed %s", message)

	return response(200, message+"\n")
}
