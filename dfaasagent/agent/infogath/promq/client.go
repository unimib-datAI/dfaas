// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package is for communicating with Prometheus.
package promq

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

type Client struct {
	// Client and API objects required by Prometheus library.
	promClient api.Client
	promAPI    v1.API

	// Query resolution size for range queries.
	promStep time.Duration

	logger *zap.SugaredLogger
}

func New(host string, port uint, step time.Duration) (*Client, error) {
	origin := fmt.Sprintf("http://%s:%d", host, port)

	client := Client{}

	promClient, err := api.NewClient(api.Config{Address: origin})
	if err != nil {
		return nil, fmt.Errorf("creating new Prometheus client: %w", err)
	}
	client.promClient = promClient

	client.promAPI = v1.NewAPI(promClient)

	if step == 0 {
		return nil, errors.New("Prometheus step must be non zero")
	}
	client.promStep = step

	client.logger = logging.Logger()

	return &client, nil
}

// Replicas returns the average number of replicas for each function in the
// given time range, always rounding up.
func (c *Client) Replicas(start, end time.Time) (map[string]uint, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}

	// We need a relative time (e.g. 1h30m) to make the range query.
	duration := end.Sub(start).String()
	query := fmt.Sprintf(`avg by (function_name) (avg_over_time(gateway_service_count[%s]))`, duration)

	// Run query.
	ctx := context.Background()
	// Instant query at 'end' time
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get replicas from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	// The query returns a single value per time series (function), so we are
	// sure that must be a Vector.
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", model.Vector{})
	}

	replicas := make(map[string]uint)
	for _, sample := range vector {
		// We know that function_name are in the form "function.namespace",
		// where usually namespace is "default". We do not want it.
		function := strings.Split(string(sample.Metric["function_name"]), ".")[0]

		// Be pessimistic: always round up!
		replicas[function] = uint(math.Ceil(float64(sample.Value)))
	}

	return replicas, nil
}

// CPUUsage returns the average CPU usage (normalized in [0, 1]) per container
// between start and end. Missing containers are set to 0 and warned.
func (c *Client) CPUUsage(start, end time.Time, containers []string) (map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}
	if len(containers) == 0 {
		return nil, errors.New("containers list cannot be empty")
	}

	// Same as Replicas().
	duration := end.Sub(start)

	// Query mainly taken from k8s/scripts/prometheus2csv/metrics.csv
	containerRegex := strings.Join(containers, "|")
	query := fmt.Sprintf(`
avg by (container) (
  avg_over_time(
    (
      irate(container_cpu_usage_seconds_total{
        namespace="default",
        container=~"%s",
        container!=""
      }[1m])
      / on(instance) group_left machine_cpu_cores
    )[%s]
  )
)`, containerRegex, duration.String())

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get CPU usage from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	// See Replicas().
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", model.Vector{})
	}

	// Initialize return value.
	usage := make(map[string]float32, len(containers))
	found := make(map[string]bool, len(containers))
	for _, c := range containers {
		usage[c] = 0
		found[c] = false
	}

	for _, sample := range vector {
		// Note in this case there is no namespace in container name.
		container := string(sample.Metric["container"])
		usage[container] = float32(sample.Value)
		found[container] = true
	}

	// Warn for missing containers.
	for _, cont := range containers {
		if !found[cont] {
			c.logger.Warnf("no CPU data for container %q in given time range", cont)
		}
	}

	return usage, nil
}
