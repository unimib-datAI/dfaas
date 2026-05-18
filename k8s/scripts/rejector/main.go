// This small program, called "rejector", is a lightweight web server that
// listens on "localhost:8080" and rejects all incoming HTTP requests.
//
// The response status code is determined by the REJECTOR_HTTP_STATUS_CODE
// environment variable. If the variable is not set, it defaults to 403
// Forbidden.
//
// In addition to the response, it adds debug headers in a style similar to
// OpenFaaS, including: X-Served-By, X-Start-Time, and X-Duration-Seconds.
//
// The main purpose of this program is to act as a "dummy" backend service in an
// HAProxy configuration, allowing rejected requests to be counted and exported
// as Prometheus metrics via HAProxy.
//
// Note that these rejections are intentional and part of the DFaaS agent
// behavior, not a result of node overload or failure conditions.
package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const addr = "0.0.0.0:8080"

func main() {
	execName := filepath.Base(os.Args[0])

	// Default status code if not given.
	statusCode := http.StatusForbidden
	if val := os.Getenv("REJECTOR_HTTP_STATUS_CODE"); val != "" {
		code, err := strconv.Atoi(val)
		if err != nil {
			fmt.Printf("Invalid REJECTOR_HTTP_STATUS_CODE: %s\n", err)
			os.Exit(1)
		}
		if code < 100 || code > 599 {
			fmt.Printf("REJECTOR_HTTP_STATUS_CODE %d out of valid HTTP range (100-599)\n", code)
			os.Exit(1)
		}
		statusCode = code
	}

	rejectAll := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Served-By", execName)
		w.Header().Set("X-Start-Time", fmt.Sprintf("%d", start.UnixNano()))
		w.Header().Set("X-Duration-Seconds", fmt.Sprintf("%.6f", time.Since(start).Seconds()))

		w.WriteHeader(statusCode)
		fmt.Fprintln(w, "Rejected directly")
	}

	http.HandleFunc("/", rejectAll)

	fmt.Println("Listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Println("Server error:", err)
	}
}
