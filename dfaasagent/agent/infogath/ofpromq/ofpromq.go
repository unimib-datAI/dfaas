// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This package is for communicating with Prometheus. The name of the package
// stands for: OpenFaas PROMetheus Querent.
package ofpromq

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/constants"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)

// Query executes a Prometheus query and returns the JSON string.
func Query(query string) (string, error) {
	logger := logging.Logger()

	strURL := fmt.Sprintf("http://%s/api/v1/query", constants.PrometheusOrigin)

	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", strURL, nil)
	if err != nil {
		logger.Error("Failed to build HTTP request for Prometheus query: ", err)
		return "", fmt.Errorf("building HTTP request: %w", err)
	}

	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	logger.Debug("Full URL for Prometheus query: ", req.URL.String())

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Prometheus HTTP request failed: ", err)
		return "", fmt.Errorf("performing HTTP request: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("Prometheus query response status: ", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read HTTP response body: ", err)
		return "", fmt.Errorf("reading response body: %w", err)
	}

	// Avoid in production!
	logger.Debug(fmt.Sprintf("Body preview: %.200s...\n", string(body)))

	return string(body), nil
}

//////////////// OPENFAAS GATEWAY METRICS QUERY ////////////////////

// queryRate performs a custom AFET rate(...) Prometheus query. The returned map has
// function names as keys average execution times as values.
func queryAFETrate(query string) (map[string]float64, error) {
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj afetResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]float64{}
	for _, item := range respObj.Data.Result {
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		result[item.Metric.FunctionName] = num
	}

	return result, nil
}

// Query that return invocation rate for each function.
//
// The returned map contain for each function (key) the returned status code
// (other key) and the invocation rate as value.
func queryInvocRate(query string) (map[string]map[string]float64, error) {
	logger := logging.Logger()

	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj invocRateResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]map[string]float64{}
	for _, item := range respObj.Data.Result {
		//funcName := strings.TrimSuffix(item.Metric.FunctionName, ".default")
		_, present := result[item.Metric.FunctionName]
		if !present {
			result[item.Metric.FunctionName] = make(map[string]float64)
		}
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		logger.Debug(item.Metric.FunctionName)
		logger.Debug(item.Metric.Code)
		result[item.Metric.FunctionName][item.Metric.Code] = num
	}

	logger.Debugf("=======================================")
	for key, codeRates := range result {
		for code, val := range codeRates {
			logger.Debugf("  - FUNC %s, CODE %s: %.2f req/s", key, code, val)
		}
	}
	logger.Debugf("=======================================")

	return result, nil
}

// This query return number of currently actived services for each function.
func queryServiceCount(query string) (map[string]int, error) {
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj serviceCountResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]int{}
	for _, item := range respObj.Data.Result {
		num, _ := strconv.Atoi(item.Value[1].(string))
		//if err != nil {
		//	num = math.NaN()
		//}
		result[item.Metric.FunctionName] = num
	}

	return result, nil
}

//////////////// NODE EXPORTER METRICS QUERY ////////////////////

// This query return the CPU usage for each specific function.
func queryCPUusage(query string) (map[string]float64, error) {
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj cpuUsageResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]float64{}
	for _, item := range respObj.Data.Result {
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		result[item.Metric.Instance] = num
	}

	return result, nil
}

// This query return the RAM usage for each specific function.
func queryRAMusage(query string) (map[string]float64, error) {
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj ramUsageResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]float64{}
	for _, item := range respObj.Data.Result {
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		result[item.Metric.Instance] = num
	}

	return result, nil
}

//////////////// cADVISOR METRICS QUERY ////////////////////

// This query return the CPU usage for each specific function.
func queryCPUusagePerFunction(query string) (map[string]float64, error) {
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj perFunctionCpuUsageResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]float64{}
	for _, item := range respObj.Data.Result {
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		result[item.Metric.ServiceName] = num
	}

	return result, nil
}

// This query return the RAM usage for each specific function.
func queryRAMusagePerFunction(query string) (map[string]float64, error) {
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj perFunctionRamUsageResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, fmt.Errorf("Error while deserializing a JSON string from the Prometheus API endpoint: %w", err)
	}

	result := map[string]float64{}
	for _, item := range respObj.Data.Result {
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		result[item.Metric.ServiceName] = num
	}

	return result, nil
}

// QueryAFET returns, for each function, the Average Function Execution Time (in
// seconds) as measured over the specified time span.
//
// The returned map has function names as keys.
//
// Warning: the time might be NaN if the values are too low.
//
// It uses metrics from the OpenFaaS Gateway.
func QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())

	rawQuery := `rate(gateway_functions_seconds_sum[%s]) / rate(gateway_functions_seconds_count[%s])`

	query := fmt.Sprintf(rawQuery, strTimeSpan, strTimeSpan)

	return queryAFETrate(query)
}

// QueryInvoc returns, for each function, the total invocation count as measured
// over the specified time span.
//
// The returned map has function names as keys.
//
// Warning: the time might be NaN if the values are too low.
//
// It uses metrics from the OpenFaaS Gateway.
func QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error) {
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())

	query := fmt.Sprintf(`rate(gateway_function_invocation_total[%s])`, strTimeSpan)

	return queryInvocRate(query)
}

// QueryServiceCount returns, for each function, the total number of function
// replicas that are currently running.
//
// The returned map has function names as keys.
//
// It uses metrics from the OpenFaaS Gateway.
func QueryServiceCount() (map[string]int, error) {
	return queryServiceCount("gateway_service_count")
}

// QueryCPUusage returns the amount of CPU used in that node.
//
// The returned map contains as keys the instance name (there may be multiple
// instances of node-exporer) and the CPU usage (percentage) as value.
//
// It uses metrics from the Prometheus node-exporter.
func QueryCPUusage(timeSpan time.Duration) (map[string]float64, error) {
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())

	// Note that the service label is optional since this metrics is exposed
	// only by Prometheus node-exporter.
	rawQuery := `1 - (avg by (instance) (rate(node_cpu_seconds_total{service="prometheus-prometheus-node-exporter", mode="idle"}[%s])))`

	query := fmt.Sprintf(rawQuery, strTimeSpan)

	return queryCPUusage(query)
}

// QueryRAMusage returns the amount of RAM used in that node.
//
// The returned map contains as keys the instance name (there may be multiple
// instances of node-exporter) and the RAM usage (percentage) as value.
//
// It uses metrics from the Prometheus node-exporter.
func QueryRAMusage(timeSpan time.Duration) (map[string]float64, error) {
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())

	rawQuery := `(
                    1 - ((
                            avg_over_time(node_memory_MemFree_bytes[%s]) +
                            avg_over_time(node_memory_Cached_bytes[%s]) +
                            avg_over_time(node_memory_Buffers_bytes[%s])
                        )
                        /
                        avg_over_time(node_memory_MemTotal_bytes[%s])
                        )
                )
                `

	query := fmt.Sprintf(rawQuery, strTimeSpan, strTimeSpan, strTimeSpan, strTimeSpan)

	// Force the rawQuery to be a single line query.
	query = strings.Join(strings.Fields(query), " ")

	return queryRAMusage(query)
}

// QueryCPUusagePerFunction returns, for each function active instance, the
// amount of CPU used by that function (avg on function containers - same number
// returned by service count).
//
// The returned map contains as keys the function name and the CPU usage
// (percentage) as value.
//
// Note: this function use metrics of cAdvisor (CPU usage of single container)
// and node-exporter (total amount of available CPU).
func QueryCPUusagePerFunction(timeSpan time.Duration, funcName []string) (map[string]float64, error) {
	if len(funcName) == 0 {
		logging.Logger().Warn("Empty funcName in QueryCPUusagePerFunction, returning empty map")
		return map[string]float64{}, nil
	}

	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	funcFilter := strings.Join(funcName, "|")

	rawQuery := `
		sum by (container) (
			irate(container_cpu_usage_seconds_total{container=~"%s"}[%s])
		)
		/ on() group_left()
		sum by (instance) (
			irate(node_cpu_seconds_total{service="prometheus-prometheus-node-exporter"}[%s])
		)
	`

	query := fmt.Sprintf(rawQuery, funcFilter, strTimeSpan, strTimeSpan)

	// Force the rawQuery to be a single line query.
	query = strings.Join(strings.Fields(query), " ")

	return queryCPUusagePerFunction(query)
}

// QueryRAMusagePerFunction returns, for each function active instance, the
// amount of RAM used by that function on total amount on available RAM (avg on
// function containers - same number returned by service count).
//
// The returned map contains as keys the function name and the RAM usage
// (percentage) as value.
//
// Note: this function use metrics of cAdvisor (RAM usage of single container)
// and node_exporter (total amount of available RAM).
func QueryRAMusagePerFunction(timeSpan time.Duration, funcName []string) (map[string]float64, error) {
	if len(funcName) == 0 {
		logging.Logger().Warn("Empty funcName in QueryCPUusagePerFunction, returning empty map")
		return map[string]float64{}, nil
	}

	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	funcFilter := strings.Join(funcName, "|")

	rawQuery := `
		(
			sum (
				avg_over_time(container_memory_usage_bytes{container=~"%s"}[%s])
			)
			by(container)
		)
		/ on() group_left()
		(
			avg_over_time(node_memory_MemTotal_bytes[%s])
		)
	`

	query := fmt.Sprintf(rawQuery, funcFilter, strTimeSpan, strTimeSpan)

	// Force the rawQuery to be a single line query.
	query = strings.Join(strings.Fields(query), " ")

	return queryRAMusagePerFunction(query)
}
