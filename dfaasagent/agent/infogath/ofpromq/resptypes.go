package ofpromq

// This file contains all structure that represents Prometheus query responses.
// Some of them are similar to each other, but they are kept separated for
// semplicity and modificability. If we want to simply change response query
// field it is necessary only to modify the struct associated to the specific
// query.

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

// Structure that represent CPU usage (per-function) cAdvisor response.
type perFunctionCpuUsageResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				ServiceName string `json:"container"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Structure that represent RAM usage (per-function) cAdvisor response.
type perFunctionRamUsageResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				ServiceName string `json:"container"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
