package proxy

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RuntimeAPI struct {
	addr    string
	timeout time.Duration
}

func NewRuntimeAPI(addr string) *RuntimeAPI {
	return &RuntimeAPI{
		addr:    addr,
		timeout: 3 * time.Second,
	}
}

func (r *RuntimeAPI) exec(cmd string) (string, error) {
	conn, err := net.DialTimeout("tcp", r.addr, r.timeout)
	if err != nil {
		return "", fmt.Errorf("connect to runtime API (%s): %w", r.addr, err)
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
		return "", fmt.Errorf("send command %q: %w", cmd, err)
	}

	reader := bufio.NewReader(conn)
	var out strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}

		// stop on empty line (common protocol delimiter)
		if line == "\n" || line == "\r\n" {
			break
		}

		out.WriteString(line)
	}

	return strings.TrimSpace(out.String()), nil
}

func (r *RuntimeAPI) Weight(backend, server string) (uint, error) {
	cmd := fmt.Sprintf("get weight %s/%s", backend, server)

	resp, err := r.exec(cmd)
	if err != nil {
		return 0, err
	}

	if resp == "No such server." {
		return 0, fmt.Errorf("server does not exist")
	}

	// Example response: "50 (initial 1)" or "No such server.".
	fields := strings.Fields(resp)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty response")
	}

	w64, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing raw content %q as integer: %w", resp, err)
	}

	// Weight is always non-negative, anyway we test that.
	if w64 < 0 {
		return 0, fmt.Errorf("negative weight %f returned", w64)
	}

	return uint(w64), nil
}

func (r *RuntimeAPI) SetWeight(backend, server string, weight uint) error {
	cmd := fmt.Sprintf("set server %s/%s weight %d", backend, server, weight)

	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("failed to set weight: %w", err)
	}

	// We ask for current weight to be sure the previous command ran
	// successfully.
	got, err := r.Weight(backend, server)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if got != weight {
		return fmt.Errorf("weight mismatch after verification: expected %d got %d", weight, got)
	}

	return nil
}

// Backends returns the sorted list of available backend names.
func (r *RuntimeAPI) Backends() ([]string, error) {
	resp, err := r.exec("show backend")
	if err != nil {
		return nil, fmt.Errorf("list backends: %w", err)
	}

	var backends []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		// We must skip empty lines and comment lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		backends = append(backends, line)
	}

	sort.Strings(backends)
	return backends, nil
}

// ReqRate returns the number of HTTP requests processed over the last second
// for each backend and server.
//
// The first level key is the backend name, the second level is the server name.
// The "BACKEND" server name is the aggregated stats for all servers under the
// same backend.
func (r *RuntimeAPI) ReqRate() (map[string]map[string]float32, error) {
	// This will return the stats in CSV format. See the following links for
	// more information.
	//
	// High level docs: https://www.haproxy.com/documentation/haproxy-runtime-api/reference/show-stat/#csv-format
	// Fields docs: https://docs.haproxy.org/3.4/management.html#9.1
	resp, err := r.exec("show stat")
	if err != nil {
		return nil, fmt.Errorf("get HAProxy stats: %w", err)
	}

	result := make(map[string]map[string]float32)

	reader := csv.NewReader(strings.NewReader(resp))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read stats header: %w", err)
	}

	// HAProxy prefixes the CSV header with "# ".
	header[0] = strings.TrimSpace(strings.TrimPrefix(header[0], "#"))

	columns := make(map[string]int, len(header))
	for i, name := range header {
		columns[strings.TrimSpace(name)] = i
	}

	// I'm only interested on a limited subset of the returned stats. Among the
	// 100 and more fields, I'm only interested on these three: the backend
	// name, the server name (or "BACKEND" for aggregated) and the number of
	// requests per second over last elapsed second.
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse stats CSV: %w", err)
		}

		get := func(name string) string {
			i := columns[name]
			if i >= len(row) {
				return ""
			}
			return row[i]
		}

		backend := get("pxname")
		server := get("svname")
		if backend == "" || server == "" || server == "FRONTEND" {
			continue
		}

		// The rate could be empty "", in that case we set 0 by default.
		rawRate := strings.TrimSpace(get("rate"))
		var rate float64
		if rawRate != "" {
			rate, err = strconv.ParseFloat(rawRate, 32)
			if err != nil {
				return nil, fmt.Errorf("parse req_rate for %s/%s: %w", backend, server, err)
			}
		}

		if result[backend] == nil {
			result[backend] = make(map[string]float32)
		}
		result[backend][server] = float32(rate)
	}

	return result, nil
}

// Variable returns the value of the given HAProxy process variable as a string.
//
// The variable argument is the name without the "proc." scope prefix. The
// returned bool indicates whether the variable was found.
func (r *RuntimeAPI) Variable(variable string) (string, bool, error) {
	cmd := fmt.Sprintf("get var proc.%s", variable)
	resp, err := r.exec(cmd)
	if err != nil {
		return "", false, fmt.Errorf("get variable %q: %w", variable, err)
	}
	resp = strings.TrimSpace(resp)

	if resp == "Variable not found." {
		return "", false, nil
	}

	// Example response: "proc.k6_iteration: type=sint value=<3>". We are
	// interested only for the section around "<>". We also ignore the variable
	// type.
	const marker = "value=<"
	start := strings.Index(resp, marker) + len(marker)
	end := strings.LastIndex(resp, ">")

	if start < 0 || end < start {
		return "", false, fmt.Errorf("unexpected response for variable %q: %s", variable, resp)
	}

	return resp[start:end], true, nil
}

// StickTable returns the raw output of the HAProxy Runtime API "show table"
// command.
func (r *RuntimeAPI) StickTable(name string) (string, error) {
	resp, err := r.exec(fmt.Sprintf("show table %s", strings.TrimSpace(name)))
	if err != nil {
		return "", fmt.Errorf("show stick table %q: %w", name, err)
	}

	if strings.Contains(resp, "No such table.") {
		return "", fmt.Errorf("stick table %q does not exist", name)
	}

	return resp, nil
}
