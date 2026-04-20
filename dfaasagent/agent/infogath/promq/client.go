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

	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	query := fmt.Sprintf(`avg by (function_name) (avg_over_time(gateway_service_count[%s]))`, durationStr)

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
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
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

	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

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
	)`, containerRegex, durationStr)

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
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
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

// InputRPS returns the average number of incoming client requests per second
// for each function within the specified time range. Note that requests from
// other nodes (forwarded) are not included.
func (c *Client) InputRPS(start, end time.Time) (map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}

	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	// Each function has two backends: one for local requests (function_X) and
	// one for incoming forwarded requests (function_X_forwarded). We want only
	// the first ones.
	query := fmt.Sprintf(`
	avg by (proxy) (
	  rate(haproxy_backend_http_requests_total{
		proxy=~"function_.*",
		proxy!~".*_forwarded"
	  }[%s])
	)`, durationStr)

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get input RPS from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	// We expect a vector result.
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
	}

	rps := make(map[string]float32, len(vector))

	for _, sample := range vector {
		proxy := string(sample.Metric["proxy"])

		// proxy format is "function_<name>"
		function := strings.TrimPrefix(proxy, "function_")

		rps[function] = float32(sample.Value)
	}

	return rps, nil
}

// AvgRespTimeLocal returns, for each function, the average response time (in
// milliseconds) of requests locally processed by the node over the given time
// range.
//
// Functions that are not present in the returned map indicate that the metric
// could not be computed (for example, due to no invocations in the specified
// time range).
func (c *Client) AvgRespTimeLocal(start, end time.Time) (map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}

	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	// We use the metric "gateway_functions_seconds" exported by OpenFaaS
	// Gateway. It's an histogram.
	// See: https://docs.openfaas.com/architecture/metrics/#gateway
	//
	// We do an "avg by" aggregation because the gateway may restart and produce
	// duplicate time series for the same function. Aggregating ensures we
	// return a single time series per function instead of multiple series
	// fragmented across gateway restarts.
	query := fmt.Sprintf(`
	avg by (function_name) (
	  rate(gateway_functions_seconds_sum[%[1]s])
	  /
	  rate(gateway_functions_seconds_count[%[1]s])
	)`, durationStr)

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get average response time from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
	}

	resp := make(map[string]float32, len(vector))

	for _, sample := range vector {
		// We know that function_name are in the form "function.namespace",
		// where usually namespace is "default". We do not want it.
		function := strings.Split(string(sample.Metric["function_name"]), ".")[0]

		value := float32(sample.Value) * 1000

		// Ignore invalid Prometheus outputs like NaN or Inf. This means that
		// the metric for this function could not be computed.
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			continue
		}

		resp[function] = value
	}

	return resp, nil
}

// RejectRate returns, for each function, the percentage of requests rejected by
// the node (only rejection by the local FaaS platform).
//
// The percentage is computed over the total number of requests in the specified
// time range.
func (c *Client) RejectRate(start, end time.Time) (map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}
	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	// The formula is: (total requests - good requests) / total requests
	//
	// We use increase because the rate is calculated over a custom time window,
	// not per second (as with rate()).
	//
	// We exclude incoming forwarded traffic and consider rejected requests as
	// requests with no response or with response not in 2xx status code.
	//
	// HAProxy is configured without retries or request replays, so each request
	// is expected to generate at most one response.
	//
	// The aggregation is done based on proxies (1 function = 1 proxy).
	query := fmt.Sprintf(`
	(
		sum by (proxy) (
		  increase(haproxy_server_http_requests_total{
			proxy=~"function_.*",
			proxy!~".*_forwarded",
			server="openfaas-local"
		  }[%[1]s])
		)
		-
		sum by (proxy) (
		  increase(haproxy_server_http_responses_total{
			proxy=~"function_.*",
			proxy!~".*_forwarded",
			server="openfaas-local",
			code="2xx"
		  }[%[1]s])
	  )
	)
	/
	sum by (proxy) (
	  increase(haproxy_server_http_requests_total{
		proxy=~"function_.*",
		proxy!~".*_forwarded",
		server="openfaas-local"
	  }[%[1]s])
	)`, durationStr)

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get reject rate from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
	}

	rates := make(map[string]float32, len(vector))

	for _, sample := range vector {
		proxy := string(sample.Metric["proxy"])

		// Proxy format is "function_<name>".
		function := strings.TrimPrefix(proxy, "function_")

		value := float32(sample.Value)

		// Replace NaN/Inf with 0 (no rejections). This can happen if no
		// requests have been made within the time range.
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			value = 0
		}

		// Must be a percentage.
		if value < 0 || value > 1 {
			return nil, fmt.Errorf("reject rate for function %q is not in [0, 1], but %f", function, value)
		}

		rates[function] = value
	}

	return rates, nil
}

// ForwardRPS returns the average number of incoming client requests per second
// to a DFaaS node for each function within the specified time range.
//
// The return type is a map with function names as the first level key and the
// DFaaS node as second level key.
//
// Note that the return map contains also the local DFaaS node names as
// "openfaas-local". Other nodes are named with "node_X", where X is its ID.
//
// Note that requests coming from other nodes (forwarded) are not included.
func (c *Client) ForwardRPS(start, end time.Time) (map[string]map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}
	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	// For each function, there are two backends (function_X and
	// function_X_forwarded). We focus only on the function_X backend. This
	// backend can have multiple servers: the default openfaas-local instance
	// and zero or more node_ID servers.
	//
	// Metrics are aggregated by proxy/backend and server, since HAProxy may
	// restart within the selected time range, potentially causing duplicated
	// time series.
	query := fmt.Sprintf(`
		sum by (proxy, server) (
		  rate(haproxy_server_http_requests_total{
			proxy=~"function_.*",
			proxy!~".*_forwarded",
		  }[%s])
		)`, durationStr)

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get forward RPS from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
	}

	// Structure: function name, node name and finally the average RPS.
	rps := make(map[string]map[string]float32)

	for _, sample := range vector {
		proxy := string(sample.Metric["proxy"])
		server := string(sample.Metric["server"])

		// Proxy format is "function_<name>".
		function := strings.TrimPrefix(proxy, "function_")

		// The sub-map may be not initialized yet.
		if _, exists := rps[function]; !exists {
			rps[function] = make(map[string]float32)
		}

		value := float32(sample.Value)

		// Handle NaN/Inf (can happen if no traffic in range).
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			value = 0
		}

		// There may be a negative approximation error.
		if value < 0 {
			value = 0
		}

		rps[function][server] = value
	}

	return rps, nil
}

// ForwardRejectRPS returns the average number of rejected client requests per
// second to a DFaaS node for each function within the specified time range.
//
// The return type is a map with function names as the first level key and the
// DFaaS node as second level key.
//
// The return map contains also the local DFaaS node names as "openfaas-local".
// Other nodes are named with "node_X", where X is its ID.
//
// Rejected requests coming from other nodes (forwarded) are not included.
func (c *Client) ForwardRejectRPS(start, end time.Time) (map[string]map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}
	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	// As with ForwardRate(), each function exposes two backends: function_X and
	// function_X_forwarded. We focus only on the function_X backend. This
	// backend has multiple servers: the default openfaas-local instance, plus
	// zero or more node_ID servers.
	//
	// We define successful requests using haproxy_server_http_responses_total
	// with 2xx status codes. Rejected requests include 4xx/5xx responses, as
	// well as cases with no response (e.g. timeouts or errors).
	//
	// HAProxy is configured without retries or request replays, so each request
	// is expected to generate at most one response.
	//
	// Metrics are aggregated by proxy/backend and server, since HAProxy may
	// restart within the selected time range, which can lead to duplicated time
	// series.
	query := fmt.Sprintf(`
	sum by (proxy, server) (
	  rate(haproxy_server_http_requests_total{
		proxy=~"function_.*",
		proxy!~".*_forwarded"
	  }[%[1]s])
	)
	-
	sum by (proxy, server) (
	  rate(haproxy_server_http_responses_total{
		proxy=~"function_.*",
		proxy!~".*_forwarded",
		code="2xx"
	  }[%[1]s])
	)`, durationStr)

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get forward reject RPS from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
	}

	// Structure: function name, node name and finally the average rps.
	rps := make(map[string]map[string]float32)

	for _, sample := range vector {
		proxy := string(sample.Metric["proxy"])
		server := string(sample.Metric["server"])

		// Proxy format is "function_<name>".
		function := strings.TrimPrefix(proxy, "function_")

		if _, exists := rps[function]; !exists {
			rps[function] = make(map[string]float32)
		}

		value := float32(sample.Value)

		// Handle NaN/Inf (can happen if no traffic in range).
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			value = 0
		}

		// There may be a negative approximation error.
		if value < 0 {
			value = 0
		}

		rps[function][server] = value
	}

	return rps, nil
}

// AvgRespTimeForward returns, for each function, the average response time (in
// milliseconds) of requests forwarded to other DFaaS nods over the given time
// range.
//
// The return type is a map with function names as the first level key and the
// DFaaS node as second level key.
//
// Note that the return map contains also the local DFaaS node names as
// "openfaas-local". Other nodes are named with "node_X", where X is its ID.
//
// Note that requests coming from other nodes (forwarded) are not included.
func (c *Client) AvgRespTimeForward(start, end time.Time) (map[string]map[string]float32, error) {
	if end.Before(start) {
		return nil, errors.New("end time must be after start time")
	}
	// Compute range duration (same approach as other functions).
	duration := end.Sub(start)

	// duration is a time.Duration. Prometheus do not supports float duration,
	// only integer. So: convert to seconds, round, then rebuild a clean duration.
	durationStr := (time.Duration(math.Round(duration.Seconds())) * time.Second).String()

	// As with other queries, each function exposes two backends: function_X and
	// function_X_forwarded. We focus only on the function_X backend. This
	// backend has multiple servers: the default openfaas-local instance, plus
	// zero or more node_ID servers.
	//
	// Metrics are aggregated by proxy/backend and server, since HAProxy may
	// restart within the selected time range, which can lead to duplicated time
	// series.
	query := fmt.Sprintf(`
	sum by (proxy, server) (
	  last_over_time(
	    haproxy_server_response_time_average_seconds{
	      proxy=~"function_.*",
	      proxy!~".*_forwarded"
	    }[%s]
	  )
	)`, durationStr)

	ctx := context.Background()
	result, warnings, err := c.promAPI.Query(ctx, query, end)
	if err != nil {
		return nil, fmt.Errorf("get forward average response time from Prometheus: %w", err)
	}
	if len(warnings) > 0 {
		c.logger.Warnf("Prometheus warnings for query %q\n%s", query, strings.Join(warnings, "\n"))
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("result type is %T, expected %T", result, model.Vector{})
	}

	// Structure: function name, node name and finally the response time (ms).
	resp := make(map[string]map[string]float32)

	for _, sample := range vector {
		proxy := string(sample.Metric["proxy"])
		server := string(sample.Metric["server"])

		// Proxy format is "function_<name>".
		function := strings.TrimPrefix(proxy, "function_")

		if _, exists := resp[function]; !exists {
			resp[function] = make(map[string]float32)
		}

		value := float32(sample.Value) * 1000

		// Ignore invalid values.
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			value = 0
		}

		// Guard against negative approximation errors.
		if value < 0 {
			value = 0
		}

		resp[function][server] = value
	}

	return resp, nil
}
