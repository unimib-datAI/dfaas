// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0-or-later license. See LICENSE and
// AUTHORS file for more information.

package openwhisk

// owAFETResponse is the Prometheus response for openwhisk_action_duration_seconds_*.
type owAFETResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Action string `json:"action"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// owInvocResponse is the Prometheus response for openwhisk_action_activations_total.
// OpenWhisk uses "status" label ("success", "developer_error", "internal_error")
// instead of an HTTP code.
type owInvocResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Action string `json:"action"`
				Status string `json:"status"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// owServiceCountResponse is the Prometheus response for kube_deployment_status_replicas.
type owServiceCountResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Deployment string `json:"deployment"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// owNodeMetricResponse is the generic Prometheus response for node-level metrics.
// The metric field name varies (instance, node, etc.); we capture it as a map.
type owNodeMetricResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// owPerFunctionMetricResponse is the Prometheus response for per-function
// cAdvisor metrics. The "container" label is used as the function/service name.
type owPerFunctionMetricResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Container string `json:"container"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
