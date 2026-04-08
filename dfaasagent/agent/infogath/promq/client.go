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
