package proxy

import (
	"bufio"
	"fmt"
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
		return 0, fmt.Errorf("negative weight %d returned", backend, server, w64)
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
