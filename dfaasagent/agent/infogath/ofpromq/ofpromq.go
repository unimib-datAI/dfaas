package ofpromq

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// This package is for communicating with a Prometheus instance of an OpenFaaS
// cluster. The name of the package stands for: OpenFaas PROMetheus Querent.

// Client for gathering information from Prometheus
type Client struct {
	Hostname string
	Port     uint
}

// Query executes a Prometheus query and returns the JSON string. The
// hostnameAndPort parameter can be like "myhostname:9090" or "myhostname"
// (implicit port 80) "192.168.15.101:9090" (specifying the IP address)
func (client *Client) Query(query string) (string, error) {
	strURL := fmt.Sprintf("http://%s:%d/api/v1/query", client.Hostname, client.Port)

	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", strURL, nil)
	if err != nil {
		return "", errors.Wrap(err, "Error while building an HTTP request for the Prometheus API endpoint")
	}

	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "Error while performing an HTTP request to the Prometheus API endpoint")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Error while reading the content of an HTTP response from the Prometheus API endpoint")
	}

	return string(body), nil
}

// rateResponse is the structure of a response from Prometheus for a rate(...) query
type rateResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				FunctionName string `json:"function_name"`
				Instance     string `json:"instance"`
				Job          string `json:"job"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

/*
	Example of Prometheus' responses for the "rate(gateway_functions_seconds_sum[20s]) / rate(gateway_functions_seconds_count[20s])" query:

	{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": []
		}
	}

	{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {
						"function_name": "funca",
						"instance": "10.0.1.24:8082",
						"job": "gateway"
					},
					"value": [
						1603286615.13,
						"NaN"
					]
				},
				{
					"metric": {
						"function_name": "funcb",
						"instance": "10.0.1.24:8082",
						"job": "gateway"
					},
					"value": [
						1603286615.13,
						"NaN"
					]
				},
				{
					"metric": {
						"function_name": "funcc",
						"instance": "10.0.1.24:8082",
						"job": "gateway"
					},
					"value": [
						1603286615.13,
						"NaN"
					]
				}
			]
		}
	}
*/

// queryRate performs a custom rate(...) Prometheus query. The returned map has
// function names as keys
func (client *Client) queryRate(query string) (map[string]float64, error) {
	strJSON, err := client.Query(query)
	if err != nil {
		return nil, err
	}

	var respObj rateResponse
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

// QueryAFET returns, for each function, the Average Function Execution Time (in
// seconds) as measured over the specified time span. The returned map has
// function names as keys
func (client *Client) QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	strTimeSpan := timeSpan.String()
	query := fmt.Sprintf("rate(gateway_functions_seconds_sum[%s]) / rate(gateway_functions_seconds_count[%s])", strTimeSpan, strTimeSpan)
	return client.queryRate(query)
}

// QueryInvoc returns, for each function, the total invocation count as measured
// over the previous time span. The returned map has function names as keys
func (client *Client) QueryInvoc(timeSpan time.Duration) (map[string]float64, error) {
	strTimeSpan := timeSpan.String()
	query := fmt.Sprintf("rate(gateway_function_invocation_total[%s])", strTimeSpan)
	return client.queryRate(query)
}
