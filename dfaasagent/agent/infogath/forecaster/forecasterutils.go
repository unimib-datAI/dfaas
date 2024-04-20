package forecaster

import (
	"strings"
)

// Available endpoints on Forecaster
const root_endpoint = ""
const node_usage_endpoint = "node_usage"
const cpu_usage_endpoint = "cpu_usage_node"
const ram_usage_endpoint = "ram_usage_node"
const power_usage_endpoint = "power_usage_node"

// Struct to read functions groups from group_list.json
type Groups struct {
	HighUsage 	[]string `json:"HIGH_USAGE"`
	MediumUsage []string `json:"MEDIUM_USAGE"`
	LowUsage 	[]string `json:"LOW_USAGE"`
}

// Struct for a request to Forecaster
type PredictionRequest struct {
	Rate_group_HIGH_USAGE 	float64 `json:"rate_group_HIGH_USAGE"`
	Rate_group_LOW_USAGE 	float64 `json:"rate_group_LOW_USAGE"`
	Rate_group_MEDIUM_USAGE float64 `json:"rate_group_MEDIUM_USAGE"`
	Node_type 				int		`json:"node_type"`
}

// Utility function used to check if a certain function is contained in a functions group
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

// Transform functions rates in functions groups rates
func SetRequestGroupsRates(funcRates map[string]float64, funcGroups Groups, req *PredictionRequest) {
	req.Rate_group_HIGH_USAGE = 0
	req.Rate_group_LOW_USAGE = 0
	req.Rate_group_MEDIUM_USAGE = 0

	for funcName, rate := range funcRates {
		clearFuncName := strings.ReplaceAll(funcName, ".openfaas-fn", "")
		if contains(funcGroups.HighUsage, clearFuncName) {
			req.Rate_group_HIGH_USAGE += rate
		} else if contains(funcGroups.MediumUsage, clearFuncName) {
			req.Rate_group_MEDIUM_USAGE += rate
		} else if contains(funcGroups.LowUsage, clearFuncName) {
			req.Rate_group_LOW_USAGE += rate
		}
	}
}
