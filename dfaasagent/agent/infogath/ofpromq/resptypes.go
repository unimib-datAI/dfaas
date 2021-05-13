package ofpromq

// This file contains all structure that represents Prometheus query responses.
// Some of them are similar to each other, but they are kept separated for semplicity and
// modifiabiliry. If we want to simply change response query field it is necessary only to
// modify the struct associated to the specific query.

// afetRespsonse is the structure of a response from Prometheus for a AFET query.
// AFET is the average execution time of invocation
type afetResponse struct {
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

// Structure that represent the invocationRate gateway query response.
type invocRateResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Code         string `json:"code"`
				FunctionName string `json:"function_name"`
				Instance     string `json:"instance"`
				Job          string `json:"job"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Structure taht represent service count gateway query response.
type serviceCountResponse struct {
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

// Structure that represent CPU usage node_exporter metric response.
type cpuUsageResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Instance string `json:"instance"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Structure that represent RAM usage node_exporter metric response.
type ramUsageResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Instance string `json:"instance"`
				Job      string `json:"job"`
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
