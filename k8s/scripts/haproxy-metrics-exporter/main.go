package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var haproxyRegex = regexp.MustCompile(`haproxy\[\d+\]:\s+(.*)`)

type LogEntry struct {
	Backend          string
	Server           string
	StatusCode       string
	TotalTimeSeconds float64
}

var haproxyLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "haproxy_custom_total_request_duration_seconds",
	Help: "Total request (with response if available) time ('Ta' in HAProxy). Custom metrics exported from HAProxy logs.",
},
	[]string{"backend", "server", "status"},
)

func init() {
	prometheus.MustRegister(haproxyLatency)
}

func parseHAProxy(line string) (LogEntry, error) {
	m := haproxyRegex.FindStringSubmatch(line)
	if len(m) < 2 {
		return LogEntry{}, fmt.Errorf("not haproxy")
	}

	parts := strings.Fields(m[1])
	if len(parts) < 6 {
		return LogEntry{}, fmt.Errorf("bad format")
	}

	// Example: "function_sentimentanalysis/openfaas-local": We split the
	// backend and server names.
	beSrv := parts[3]
	beParts := strings.Split(beSrv, "/")
	if len(beParts) != 2 {
		return LogEntry{}, fmt.Errorf("bad backend/server")
	}

	// Example: "0/0/0/855/855". We are intested on the last value, the total
	// active time (called also 'Ta' in HAProxy). They're always milliseconds.
	timings := strings.Split(parts[4], "/")
	totalMsStr := timings[len(timings)-1]
	totalTime, err := time.ParseDuration(totalMsStr + "ms")
	if err != nil {
		return LogEntry{}, fmt.Errorf("failed to parse Ta timing: %w", err)
	}

	// Example: "200". Always present after the timings array.
	status := parts[5]

	return LogEntry{
		Backend:          beParts[0],
		Server:           beParts[1],
		StatusCode:       status,
		TotalTimeSeconds: totalTime.Seconds(),
	}, nil
}

func main() {
	// Parse CLI flags.
	metricsPort := flag.Int("metrics-port", 2112, "Port for Prometheus /metrics endpoint")
	logsPort := flag.Int("logs-port", 5014, "UDP port for incoming HAProxy logs")
	flag.Parse()

	// Start web server to expose Prometheus endpoint.
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		addr := fmt.Sprintf(":%d", *metricsPort)
		log.Printf("Starting Prometheus metrics server on :%d/metrics...", *metricsPort)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalln("Prometheus metrics server failed:", err)
		}
	}()

	// Start an UDP server for incoming logs.
	addr := net.UDPAddr{Port: *logsPort, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	log.Printf("Started UDP syslog server on :%d", *logsPort)

	// We hardcoded the max line size to 8192 bytes, should be enough.
	buf := make([]byte, 8192)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("Failed to read UDP datagram:", err)
			continue
		}

		line := string(buf[:n])
		entry, err := parseHAProxy(line)
		if err != nil {
			log.Printf("Skipping parsing of HAProxy raw log %q:", line, err)
			continue
		}

		haproxyLatency.WithLabelValues(
			entry.Backend,
			entry.Server,
			entry.StatusCode,
		).Observe(entry.TotalTimeSeconds)
	}
}
