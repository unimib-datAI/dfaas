package forecaster

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"github.com/pkg/errors"
)

// Package for sending requests to Forecaster to get predictions about the usage of the node.

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
func (client *Client) GetNodeUsagePredictions(funcsGroupsLoadInfo string) (string, error) {
	return client.doRequest(funcsGroupsLoadInfo, node_usage_endpoint)
}

// Function to retrieve the CPU usage prediction of the node
func (client *Client) GetCPUUsageNodePredictions(funcsGroupsLoadInfo string) (string, error) {
	return client.doRequest(funcsGroupsLoadInfo, cpu_usage_endpoint)
}

// Function to retrieve the RAM usage prediction of the node
func (client *Client) GetRAMUsageNodePredictions(funcsGroupsLoadInfo string) (string, error) {
	return client.doRequest(funcsGroupsLoadInfo, ram_usage_endpoint)
}

// Function to retrieve the power usage prediction of the node
func (client *Client) GetPowerUsageNodePredictions(funcsGroupsLoadInfo string) (string, error) {
	return client.doRequest(funcsGroupsLoadInfo, power_usage_endpoint)
}

// Function to check if server is healthy
func (client *Client) HealthCheck() (string, error) {
	return client.doRequest("", root_endpoint)
}