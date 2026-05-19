package proxy

import (
	"bufio"
	"fmt"
	"net"
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

func (r *RuntimeAPI) GetWeight(backend, server string) (uint, error) {
	cmd := fmt.Sprintf("get weight %s/%s", backend, server)

	resp, err := r.exec(cmd)
	if err != nil {
		return 0, err
	}

	// Example response: "50 (initial 1)".
	fields := strings.Fields(resp)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty response for %s/%s", backend, server)
	}

	w64, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse weight %q for %s/%s (raw: %q): %w",
			fields[0], backend, server, resp, err)
	}

	// Weight is always non-negative, anyway we test that.
	if w64 < 0 {
		return 0, fmt.Errorf("negative weight returned for %s/%s: %d", backend, server, w64)
	}

	return uint(w64), nil
}

func (r *RuntimeAPI) SetWeight(backend, server string, weight uint) error {
	cmd := fmt.Sprintf("set server %s/%s weight %d", backend, server, weight)

	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("set weight %d for %s/%s: %w", weight, backend, server, err)
	}

	// We ask for current weight to be sure the previous command ran
	// successfully.
	got, err := r.GetWeight(backend, server)
	if err != nil {
		return fmt.Errorf("verify weight for %s/%s: %w", backend, server, err)
	}

	if got != weight {
		return fmt.Errorf("weight mismatch for %s/%s: expected %d got %d",
			backend, server, weight, got)
	}

	return nil
}
