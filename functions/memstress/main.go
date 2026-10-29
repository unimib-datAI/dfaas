package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	defaultMiB     = 128
	defaultSeconds = 5
	maxMib         = 1024
	maxSeconds     = 5 * 60
)

func main() {
	http.HandleFunc("/", Handle)

	log.Println("memstress listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

// intParam reads an integer query parameter. If the parameter is absent, def is
// returned. If it is present but not an integer in [min, max], an error is
// returned.
func intParam(r *http.Request, name string, def, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return 0, fmt.Errorf("invalid %q=%q: must be an integer between %d and %d", name, raw, min, max)
	}
	return v, nil
}

func Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mb, err := intParam(r, "mb", defaultMiB, 1, maxMiB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	seconds, err := intParam(r, "seconds", defaultSeconds, 0, maxSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(message + "\n"))
}
