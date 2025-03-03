// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// Package for sending requests to Forecaster to get predictions about the usage
// metrics of the node (e.g. CPU, RAM, power consumption).
package forecaster

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"encoding/json"
	"github.com/pkg/errors"
)

// Available endpoints on Forecaster
const root_endpoint = ""
const node_usage_endpoint = "node_usage"
const cpu_usage_endpoint = "cpu_usage_node"
const ram_usage_endpoint = "ram_usage_node"
const power_usage_endpoint = "power_usage_node"

// Struct for a request to Forecaster to predict node metrics
type NodeMetricPredReq struct {
	Rate_group_HIGH_USAGE 	float64 `json:"rate_group_HIGH_USAGE"`
	Rate_group_LOW_USAGE 	float64 `json:"rate_group_LOW_USAGE"`
	Rate_group_MEDIUM_USAGE float64 `json:"rate_group_MEDIUM_USAGE"`
	Node_type 				int		`json:"node_type"`
}

// Struct for the node metrics prediction response from Forecaster
type NodeMetricPredRes struct {
	Cpu_usage_node 		float64 `json:"cpu_usage_node"`
	Ram_usage_node 		float64 `json:"ram_usage_node"`
	Power_usage_node 	float64 `json:"power_usage_node"`
}

// Client for gathering information from the Forecaster
type Client struct {
	Hostname string
	Port     uint
}

// Send a GET request to the specified endpoint of the Forecaster, with a request body (if necessary) correspondent to the string parameter "payload",
// and return a json response containing the predictions.
func (client *Client) doRequest(payload string, endpoint string) (string, error) {
	strURL := fmt.Sprintf("http://%s:%d/%s", client.Hostname, client.Port, endpoint)

	httpClient := &http.Client{}

	var req *http.Request
	var err error

	if payload != "" {
		body := strings.NewReader(payload)
		req, err = http.NewRequest("GET", strURL, body)
	} else {
		req, err = http.NewRequest("GET", strURL, nil)
	}
	if err != nil {
		return "", errors.Wrap(err, "Error while building an HTTP request for the Forecaster endpoint")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "Error while performing an HTTP request to the Forecaster endpoint")
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Error while reading the content of an HTTP response from the Forecaster endpoint")
	}

	return string(respBody), nil
}

///////////////// PUBLIC INTERFACE /////////////////////

// Function to retrieve all the node usage predictions (RAM usage, power usage, CPU usage)
func (client *Client) GetNodeUsagePredictions(request NodeMetricPredReq) (NodeMetricPredRes, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while constructing json request to Forecaster")
	}
	jsonBodyStr := string(jsonBody)

	var jsonResp string
	jsonResp, err = client.doRequest(jsonBodyStr, node_usage_endpoint)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while executing request to Forecaster")
	}

	var predictionsResponse []NodeMetricPredRes
	err = json.Unmarshal([]byte(jsonResp), &predictionsResponse)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while converting json response from Forecaster")
	}
	
	return predictionsResponse[0], nil
}

// Function to retrieve the CPU usage prediction of the node
func (client *Client) GetCPUUsageNodePredictions(request NodeMetricPredReq) (NodeMetricPredRes, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while constructing json request to Forecaster")
	}
	jsonBodyStr := string(jsonBody)

	var jsonResp string
	jsonResp, err = client.doRequest(jsonBodyStr, cpu_usage_endpoint)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while executing request to Forecaster")
	}

	var predictionsResponse []NodeMetricPredRes
	err = json.Unmarshal([]byte(jsonResp), &predictionsResponse)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while converting json response from Forecaster")
	}
	
	return predictionsResponse[0], nil
}

// Function to retrieve the RAM usage prediction of the node
func (client *Client) GetRAMUsageNodePredictions(request NodeMetricPredReq) (NodeMetricPredRes, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while constructing json request to Forecaster")
	}
	jsonBodyStr := string(jsonBody)

	var jsonResp string
	jsonResp, err = client.doRequest(jsonBodyStr, ram_usage_endpoint)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while executing request to Forecaster")
	}

	var predictionsResponse []NodeMetricPredRes
	err = json.Unmarshal([]byte(jsonResp), &predictionsResponse)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while converting json response from Forecaster")
	}
	
	return predictionsResponse[0], nil
}

// Function to retrieve the power usage prediction of the node
func (client *Client) GetPowerUsageNodePredictions(request NodeMetricPredReq) (NodeMetricPredRes, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while constructing json request to Forecaster")
	}
	jsonBodyStr := string(jsonBody)

	var jsonResp string
	jsonResp, err = client.doRequest(jsonBodyStr, power_usage_endpoint)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while executing request to Forecaster")
	}

	var predictionsResponse []NodeMetricPredRes
	err = json.Unmarshal([]byte(jsonResp), &predictionsResponse)
	if err != nil {
		return NodeMetricPredRes{}, errors.Wrap(err, "Error while converting json response from Forecaster")
	}
	
	return predictionsResponse[0], nil
}

// Function to check if Forecaster server is healthy
func (client *Client) HealthCheck() (string, error) {
	return client.doRequest("", root_endpoint)
}
