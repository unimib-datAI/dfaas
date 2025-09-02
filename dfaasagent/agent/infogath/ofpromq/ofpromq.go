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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

    "gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
    "gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
)

// Query executes a Prometheus query and returns the JSON string.
func Query(query string) (string, error) {
    logger := logging.Logger()

    strURL := fmt.Sprintf("http://%s/api/v1/query", constants.PrometheusOrigin)

	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", strURL, nil)
	if err != nil {
        logger.Error("Failed to build HTTP request for Prometheus query: ", err)
		return "", errors.Wrap(err, "building HTTP request")
	}

	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

    logger.Debug("Full URL for Prometheus query: ", req.URL.String())

	resp, err := httpClient.Do(req)
	if err != nil {
        logger.Error("Prometheus HTTP request failed: ", err)
		return "", errors.Wrap(err, "performing HTTP request")
	}
	defer resp.Body.Close()

    logger.Debug("Prometheus query response status: ", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
        logger.Error("Failed to read HTTP response body: ", err)
		return "", errors.Wrap(err, "reading response body")
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
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
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
// The returned map contain for each function (key) the returned status code (other key)
// and the invocation rate as value.
func queryInvocRate(query string) (map[string]map[string]float64, error) {
	//logger := logging.Logger()
	strJSON, err := Query(query)
	if err != nil {
		return nil, err
	}

	var respObj invocRateResponse
	err = json.Unmarshal([]byte(strJSON), &respObj)
	if err != nil {
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
	}

	//logger := logging.Logger()

	result := map[string]map[string]float64{}
	for _, item := range respObj.Data.Result {
		_, present := result[item.Metric.FunctionName]
		if !present {
			result[item.Metric.FunctionName] = make(map[string]float64)
		}
		num, err := strconv.ParseFloat(item.Value[1].(string), 64)
		if err != nil {
			num = math.NaN()
		}
		//logger.Debug(item.Metric.FunctionName)
		//logger.Debug(item.Metric.Code)
		result[item.Metric.FunctionName][item.Metric.Code] = num
	}

	//logger.Debugf("=======================================")
	//for key, codeRates := range result {
	//	for code, val := range codeRates {
	//		logger.Debugf("  - FUNC %s, CODE %s: %.2f req/s", key, code, val)
	//	}
	//}
	//logger.Debugf("=======================================")

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
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
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
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
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
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
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
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
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
		return nil, errors.Wrap(err, "Error while deserializing a JSON string from the Prometheus API endpoint")
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

///////////////// PUBLIC INTERFACE /////////////////////

// QueryAFET returns, for each function, the Average Function Execution Time (in
// seconds) as measured over the specified time span. The returned map has
// function names as keys
func QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf("rate(gateway_functions_seconds_sum[%s]) / rate(gateway_functions_seconds_count[%s])", strTimeSpan, strTimeSpan)
	//logging.Logger().Debug(query)
	return queryAFETrate(query)
}

// QueryInvoc returns, for each function, the total invocation count as measured
// over the previous time span. The returned map has function names as keys
func QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error) {
	//strTimeSpan := timeSpan.String()
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf("rate(gateway_function_invocation_total[%s])", strTimeSpan)
	return queryInvocRate(query)
}

// QueryServiceCount returns, for each function, the total number of service active for each
// function. The returned map contains function names as keys.
func QueryServiceCount() (map[string]int, error) {
	//strTimeSpan := timeSpan.String()
	//strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf("gateway_service_count")
	return queryServiceCount(query)
}

// QueryCPUusage returns, for each active istance of node_exporter, the amount of CPU used
// in that node. The returned map contains as keys the instance name and the CPU usage (percentage) as value.
func QueryCPUusage(timeSpan time.Duration) (map[string]float64, error) {
    logger := logging.Logger()

    // Check IS_KUBE env var, set to true if running on Kubernetes.
    isKube := false
	if val := os.Getenv("IS_KUBE"); val != "" {
        var err error
        isKube, err = strconv.ParseBool(val)
        if err != nil {
            logger.Error(fmt.Sprintf("Invalid IS_KUBE value: %s", val))
        }
    }

	//strTimeSpan := timeSpan.String()
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())

	query := ""

	if isKube {
		query = fmt.Sprintf("1 - (avg by (instance) (rate(node_cpu_seconds_total{job=\"node-exporter\",mode=\"idle\"}[%s])))", strTimeSpan)
	} else {
		query = fmt.Sprintf("1 - (avg by (instance) (rate(node_cpu_seconds_total{job=\"node\",mode=\"idle\"}[%s])))", strTimeSpan)
	}

	return queryCPUusage(query)
}

// QueryRAMusage returns, for each active istance of node_exporter, the amount of RAM used
// in that node. The returned map contains as keys the instance name and the RAM usage (percentage) as value.
func QueryRAMusage(timeSpan time.Duration) (map[string]float64, error) {
	//strTimeSpan := timeSpan.String()
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	/*query := fmt.Sprintf(
	`1 -
	(
		node_memory_MemAvailable_bytes or
		(
			node_memory_Buffers_bytes +
			node_memory_Cached_bytes +
			node_memory_MemFree_bytes +
			node_memory_Slab_bytes
		)
	) / node_memory_MemTotal_bytes`)*/

	query := fmt.Sprintf("(1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))", strTimeSpan, strTimeSpan, strTimeSpan, strTimeSpan)
	return queryRAMusage(query)
}

// QueryCPUusagePerFunction returns, for each function active instance, the amount of CPU used
// by that function (avg on function containers - same number returned by service count).
// The returned map contains as keys the function name and the CPU usage (percentage) as value.
// Note: this function use metrics of cAdvisor (CPU usage of single container) and node_exporter (total amount of available CPU).
func QueryCPUusagePerFunction(timeSpan time.Duration, funcName []string) (map[string]float64, error) {
    logger := logging.Logger()

    isKube := false
	if val := os.Getenv("IS_KUBE"); val != "" {
        var err error
        isKube, err = strconv.ParseBool(val)
        if err != nil {
            logger.Error(fmt.Sprintf("Invalid IS_KUBE value: %s", val))
        }
    }

	//strTimeSpan := timeSpan.String()
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	funcFilter := strings.Join(funcName, "|")

	query := ""

	if isKube {
		query = fmt.Sprintf("sum by (id) (irate(container_cpu_usage_seconds_total{id=~\".*(%s).*\"}[%s])) / on() group_left() sum by (instance) (irate(node_cpu_seconds_total{job=\"node-exporter\"}[%s]))", funcFilter, strTimeSpan, strTimeSpan)
	} else {
		query = fmt.Sprintf("sum by (id) (irate(container_cpu_usage_seconds_total{id=~\".*(%s).*\"}[%s])) / on() group_left() sum by (instance) (irate(node_cpu_seconds_total{job=\"node\"}[%s]))", funcFilter, strTimeSpan, strTimeSpan)
	}

	return queryCPUusagePerFunction(query)
}

// QueryRAMusagePerFunction returns, for each function active instance, the amount of RAM used
// by that function on total amount on available RAM (avg on function containers - same number returned by service count).
// The returned map contains as keys the function name and the RAM usage (percentage) as value.
// Note: this function use metrics of cAdvisor (RAM usage of single container) and node_exporter (total amount of available RAM).
func QueryRAMusagePerFunction(timeSpan time.Duration, funcName []string) (map[string]float64, error) {
	//strTimeSpan := timeSpan.String()
	strTimeSpan := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	funcFilter := strings.Join(funcName, "|")
	query := fmt.Sprintf("(sum (avg_over_time(container_memory_usage_bytes{id=~\".*(%s).*\"}[%s])) by(id)) / on() group_left() (avg_over_time(node_memory_MemTotal_bytes[%s]))", funcFilter, strTimeSpan, strTimeSpan)

	return queryRAMusagePerFunction(query)
}
