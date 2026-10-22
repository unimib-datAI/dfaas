// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// debugLogger returns the logger only when the Debug level is enabled.
func debugLogger() (*zap.SugaredLogger, bool) {
	logger := logging.Logger()
	if !logger.Desugar().Core().Enabled(zapcore.DebugLevel) {
		return logger, false
	}
	return logger, true
}

func debugPromCPUusage(timeSpan time.Duration, data map[string]float64) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Nodes CPU usage (over " + timeSpan.String() + " time span):\n")
	for _, instance := range keys {
		b.WriteString(fmt.Sprintf("  - Instance %s CPU utilization: %.2f%%\n", instance, data[instance]*100))
	}

	logger.Debugw("Nodes CPU usage",
		"message", b.String(),
		"time_span", timeSpan.String(),
		"instances", len(data),
	)
}

func debugPromRAMusage(timeSpan time.Duration, data map[string]float64) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Nodes RAM usage (over " + timeSpan.String() + " time span):\n")
	for _, instance := range keys {
		b.WriteString(fmt.Sprintf("  - Instance %s RAM utilization: %.2f%%\n", instance, data[instance]*100))
	}

	logger.Debugw("Nodes RAM usage",
		"message", b.String(),
		"time_span", timeSpan.String(),
		"instances", len(data),
	)
}

func debugNodesTblContentNMS(entries map[string]*nodestbl.EntryNMS) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	nodeIDs := make([]string, 0, len(entries))
	for k := range entries {
		nodeIDs = append(nodeIDs, k)
	}

	sort.Strings(nodeIDs)

	var b strings.Builder
	b.WriteString("Content of nodestbl:\n")
	for _, nodeID := range nodeIDs {
		entry := entries[nodeID]

		b.WriteString(fmt.Sprintf("  - NODE %s (HAProxy=%s:%d), type: %d, common neighbour: %t, margin: %.2f\n",
			nodeID,
			entry.HAProxyHost,
			entry.HAProxyPort,
			entry.NodeType,
			entry.CommonNeighbour,
			entry.Margin))

		b.WriteString(fmt.Sprintf("    - LOAD: highUsage=%.2f req/s, mediumhUsage=%.2f req/s, lowUsage=%.2f req/s\n",
			entry.Load.RateHighUsage, entry.Load.RateMediumUsage, entry.Load.RateLowUsage))

		b.WriteString("    - FUNCTIONS:\n")
		for _, funcName := range entry.Funcs {
			b.WriteString(fmt.Sprintf("       FUNC: %s\n", funcName))
		}
	}

	logger.Debugw("Content of nodestbl",
		"message", b.String(),
		"nodes", len(entries),
	)
}

func debugMsgNodeInfoNMS(msg MsgNodeInfoNMS) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var buf strings.Builder
	if len(msg.Functions) == 0 {
		buf.WriteString(fmt.Sprintf("MsgNodeInfoNMS content: type=%d functions=empty\n", msg.NodeType))
	} else {
		buf.WriteString(fmt.Sprintf("MsgNodeInfoNMS content: type=%d functions=%v\n", msg.NodeType, msg.Functions))
	}

	logger.Debugw("MsgNodeInfoNMS content",
		"message", buf.String(),
		"node_type", msg.NodeType,
		"functions", msg.Functions,
	)
}

func debugMsgNodeInfoStatic(msg MsgNodeInfoStatic) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var buf strings.Builder
	if len(msg.Functions) == 0 {
		buf.WriteString("MsgNodeInfoStatic content: functions=empty\n")
	} else {
		buf.WriteString(fmt.Sprintf("MsgNodeInfoStatic content: functions=%v\n", msg.Functions))
	}

	logger.Debugw("MsgNodeInfoStatic content",
		"message", buf.String(),
		"functions", msg.Functions,
	)
}

func debugFuncsLoad(load GroupsLoad) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b,
		"Node's Load: High Usage=%.2f req/s, Low Usage=%.2f req/s, Medium Usage=%.2f req/s",
		load.RateHighUsage,
		load.RateLowUsage,
		load.RateMediumUsage,
	)

	logger.Debugw("Node's Load",
		"message", b.String(),
		"high_usage", load.RateHighUsage,
		"low_usage", load.RateLowUsage,
		"medium_usage", load.RateMediumUsage,
	)
}

func debugNodeMetricPredictions(predictions map[string]float64) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var b strings.Builder
	b.WriteString("Node's usage predictions:\n")
	b.WriteString(fmt.Sprintf("CPU usage: %.2f, RAM usage: %.2f, Power usage: %.2f\n",
		predictions[cpuUsageNodeMetric],
		predictions[ramUsageNodeMetric],
		predictions[powerUsageNodeMetric]))

	logger.Debugw("Node's usage predictions",
		"message", b.String(),
		"cpu_usage", predictions[cpuUsageNodeMetric],
		"ram_usage", predictions[ramUsageNodeMetric],
		"power_usage", predictions[powerUsageNodeMetric],
	)
}

func debugOverloadNMS(overload bool) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var b strings.Builder
	if overload {
		b.WriteString("Node's state: OVERLOAD")
	} else {
		b.WriteString("Node's state: UNDERLOAD")
	}

	logger.Debugw("Node's state",
		"message", b.String(),
		"overload", overload,
	)
}

func debugNodeUsagePercentage(percentage float64) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Node usage percentage: %.2f", percentage)

	logger.Debugw("Node usage percentage",
		"message", b.String(),
		"percentage", percentage,
	)
}

func debugNodeMargin(margin float64) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Node's margin: %.2f", margin)

	logger.Debugw("Node's margin",
		"message", b.String(),
		"margin", margin,
	)
}

func debugWeightsNMS(weights map[string]map[string]uint) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	functions := make([]string, 0, len(weights))
	for fun := range weights {
		functions = append(functions, fun)
	}
	sort.Strings(functions)

	var buf strings.Builder
	buf.WriteString("Calculated weights:\n")
	for _, fun := range functions {
		fmt.Fprintf(&buf, "  - function %s:\n", fun)

		nodes := make([]string, 0, len(weights[fun]))
		for node := range weights[fun] {
			nodes = append(nodes, node)
		}
		sort.Strings(nodes)

		for _, node := range nodes {
			fmt.Fprintf(&buf, "    - Node %s: %d\n", node, weights[fun][node])
		}
	}

	logger.Debugw("Calculated weights",
		"message", buf.String(),
		"functions", len(weights),
	)
}

// debugFuncsDiff prints the difference between the current and previous
// function names map, where the key is the function name and the value is a
// pointer to uint. It also checks if the pointer values differ (including nil
// vs non-nil and value differences).
//
// Example output:
//
//	Detected deployed functions: (total 4) shasum +figlet mlimage -curl !foo=1->nil !bar=nil->3 !baz=5->8
func debugFuncsDiff(current, previous map[string]*uint) {
	logger, ok := debugLogger()
	if !ok {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Detected deployed functions: (total %d) ", len(current))

	// Collect all unique function names from both maps.
	keysSet := make(map[string]struct{})
	for k := range current {
		keysSet[k] = struct{}{}
	}
	for k := range previous {
		keysSet[k] = struct{}{}
	}

	// Collect and sort all keys.
	keys := make([]string, 0, len(keysSet))
	for k := range keysSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Check values and keys.
	for idx, k := range keys {
		cv, inCurrent := current[k]
		pv, inPrevious := previous[k]

		switch {
		case inCurrent && !inPrevious:
			// New function.
			b.WriteString("+")
			b.WriteString(k)

		case !inCurrent && inPrevious:
			// Removed function.
			b.WriteString("-")
			b.WriteString(k)

		case inCurrent && inPrevious:
			// Check values.
			if (cv == nil && pv != nil) || (cv != nil && pv == nil) ||
				(cv != nil && pv != nil && *cv != *pv) {
				// Value changed!
				b.WriteString("!")
				b.WriteString(k)
				b.WriteString("=")

				var prevStr, currStr string
				if pv == nil {
					prevStr = "nil"
				} else {
					prevStr = fmt.Sprintf("%d", *pv)
				}
				if cv == nil {
					currStr = "nil"
				} else {
					currStr = fmt.Sprintf("%d", *cv)
				}

				b.WriteString(prevStr)
				b.WriteString("->")
				b.WriteString(currStr)
			} else {
				// Not changed.
				b.WriteString(" ")
				b.WriteString(k)
			}
		}

		if idx < len(keys)-1 {
			b.WriteString(" ")
		}
	}

	logger.Debugw("Detected deployed functions",
		"message", b.String(),
		"current_count", len(current),
		"previous_count", len(previous),
	)
}
