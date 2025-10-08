// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/hasock"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
)

//////////////////// PRIVATE FUNCTIONS FOR LOGGING ////////////////////

func debugConnectedNodes(nodeIDs []peer.ID) {
	if !logging.GetDebugMode() {
		return
	}

	var b strings.Builder
	b.WriteString("Currently connected nodes:")

	if len(nodeIDs) == 0 {
		b.WriteString(" 0 nodes")
	} else {
		shortIDs := make([]string, len(nodeIDs))

		for i, nodeID := range nodeIDs {
			strNodeID := nodeID.String()
			shortIDs[i] = strNodeID[len(strNodeID)-3:]
		}

		sort.Strings(shortIDs)

		for _, shortNodeID := range shortIDs {
			b.WriteString(" " + shortNodeID)
		}
	}

	logging.Logger().Debug(b.String())
}

func debugPromAFET(timeSpan time.Duration, data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Average functions exec times (over " + timeSpan.String() + " time span):\n")
	for _, funcName := range keys {
		b.WriteString(fmt.Sprintf("  - FUNC %s: %.3f ms\n", funcName, data[funcName]*1000))
	}
	logger.Debug(b.String())
}

func debugPromInvoc(timeSpan time.Duration, data map[string]map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Functions invocation counts (over %v time span):", timeSpan))
	if len(keys) > 0 {
		b.WriteString("\n")
		for _, funcName := range keys {
			for code, rate := range data[funcName] {
				b.WriteString(fmt.Sprintf("  - FUNC %q, CODE %s: %.2f req/s\n", funcName, code, rate))
			}
		}
	} else {
		b.WriteString(" no available functions")
	}
	logging.Logger().Debug(b.String())
}

func debugPromServiceCount(data map[string]int) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Functions service counts:\n")
	for _, funcName := range keys {
		b.WriteString(fmt.Sprintf("  - FUNC %s: %d active function replicas\n", funcName, data[funcName]))
	}
	logger.Debug(b.String())
}

func debugPromCPUusage(timeSpan time.Duration, data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

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
	logger.Debug(b.String())
}

func debugPromRAMusage(timeSpan time.Duration, data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

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
	logger.Debug(b.String())
}

func debugPromCPUusagePerFunction(timeSpan time.Duration, data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Nodes CPU usage for function (over " + timeSpan.String() + " time span) and averaged on number of container:\n")
	for _, funcName := range keys {
		b.WriteString(fmt.Sprintf("  - FUNC %s CPU utilization: %.2f%%\n", funcName, data[funcName]*100))
	}
	logger.Debug(b.String())
}

func debugPromRAMusagePerFunction(timeSpan time.Duration, data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Nodes RAM usage for function (over " + timeSpan.String() + " time span) and averaged on number of container:\n")
	for _, funcName := range keys {
		b.WriteString(fmt.Sprintf("  - FUNC %s RAM utilization: %.2f%%\n", funcName, data[funcName]*100))
	}
	logger.Debug(b.String())
}

func debugHAProxyUserRates(data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("Invocation rates of requests from users only (calculated from HAProxy stick-table):")
	if len(keys) == 0 {
		b.WriteString("empty")
	} else {
		for _, funcName := range keys {
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  - FUNC %s: %.2f req/s\n", funcName, data[funcName]))
		}
	}
	logging.Logger().Debug(b.String())
}

func debugFuncs(data map[string]uint) {
	if !logging.GetDebugMode() {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Available functions: %d", len(keys)))
	if len(keys) > 0 {
		b.WriteString(" (limit req/s) ")
		for _, funcName := range keys {
			b.WriteString(fmt.Sprintf("%q (%s) ", funcName, data[funcName]))
		}
	}
	logging.Logger().Debug(b.String())
}

func debugNodesTblContent(entries map[string]*nodestbl.EntryRecalc) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	nodeIDs := make([]string, 0, len(entries))
	for k := range entries {
		nodeIDs = append(nodeIDs, k)
	}

	sort.Strings(nodeIDs)

	var b strings.Builder
	b.WriteString("Content of nodestbl:")
	if len(nodeIDs) == 0 {
		b.WriteString("empty")
		logger.Debug(b.String())
		return
	}

	b.WriteString("\n")
	for _, nodeID := range nodeIDs {
		entry := entries[nodeID]

		b.WriteString(fmt.Sprintf("  - NODE %s (HAProxy=%s:%d)\n",
			nodeID,
			entry.HAProxyHost,
			entry.HAProxyPort))

		funcNames := make([]string, 0, len(entry.FuncsData))
		for k := range entry.FuncsData {
			funcNames = append(funcNames, k)
		}

		sort.Strings(funcNames)

		for _, funcName := range funcNames {
			funcData := entry.FuncsData[funcName]

			b.WriteString(fmt.Sprintf("    - FUNC %s: LimitIn=%.2f LimitOut=%.2f NodeWeight=%d\n",
				funcName,
				funcData.LimitIn,
				funcData.LimitOut,
				funcData.NodeWeight))
		}
	}
	logger.Debug(b.String())
}

func debugNodesTblContentNMS(entries map[string]*nodestbl.EntryNMS) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

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
	logger.Debug(b.String())
}

func debugStickTable(stName string, stContent map[string]*hasock.STEntry) {
	if !logging.GetDebugMode() {
		return
	}

	clients := make([]string, 0, len(stContent))
	for k := range stContent {
		clients = append(clients, k)
	}

	sort.Strings(clients)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("HAProxy stick-table %q content:", stName))
	if len(clients) == 0 {
		b.WriteString(" empty")
	} else {
		b.WriteString("\n")
		for _, key := range clients {
			stEntry := stContent[key]
			b.WriteString(fmt.Sprintf("  - key=%s: cnt=%d rate=%d\n", key, stEntry.HTTPReqCnt, stEntry.HTTPReqRate))
		}
	}
	logging.Logger().Debug(b.String())
}

func debugMsgNodeInfoRecalc(msg MsgNodeInfoRecalc) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	var b strings.Builder
	for _nodeID, _limits := range msg.FuncLimits {
		b.WriteString(fmt.Sprintf("Functions limits for node %s:\n", _nodeID))
		for funcName := range _limits {
			b.WriteString(fmt.Sprintf("	Function %s LimitOut: %f\n", funcName, _limits[funcName]))
		}
	}
	logger.Debug(b.String())
}

func debugMsgNodeInfoNMS(msg MsgNodeInfoNMS) {
	if !logging.GetDebugMode() {
		return
	}

	var buf strings.Builder
	if len(msg.Functions) == 0 {
		buf.WriteString(fmt.Sprintf("MsgNodeInfoNMS content: type=%d functions=empty\n", msg.NodeType))
	} else {
		buf.WriteString(fmt.Sprintf("MsgNodeInfoNMS content: type=%d functions=%v\n", msg.NodeType, msg.Functions))
	}
	logging.Logger().Debug(buf.String())
}

func debugMsgNodeInfoStatic(msg MsgNodeInfoStatic) {
	if !logging.GetDebugMode() {
		return
	}

	var buf strings.Builder
	if len(msg.Functions) == 0 {
		buf.WriteString("MsgNodeInfoStatic content: functions=empty\n")
	} else {
		buf.WriteString(fmt.Sprintf("MsgNodeInfoStatic content: functions=%v\n", msg.Functions))
	}
	logging.Logger().Debug(buf.String())
}

func debugFuncsLoad(load GroupsLoad) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	logger.Debugf("Node's Load: High Usage=%.2f req/s, Low Usage=%.2f req/s, Medium Usage=%.2f req/s",
		load.RateHighUsage,
		load.RateLowUsage,
		load.RateMediumUsage)
}

func debugNodeMetricPredictions(predictions map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	var b strings.Builder
	b.WriteString("Node's usage predictions:\n")
	b.WriteString(fmt.Sprintf("CPU usage: %.2f, RAM usage: %.2f, Power usage: %.2f\n",
		predictions[cpuUsageNodeMetric],
		predictions[ramUsageNodeMetric],
		predictions[powerUsageNodeMetric]))
	logger.Debug(b.String())
}

func debugOverloadNMS(overload bool) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	if overload {
		logger.Debugf("Node's state: OVERLOAD")
	} else {
		logger.Debugf("Node's state: UNDERLOAD")
	}
}

func debugNodeUsagePercentage(percentage float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	logger.Debugf("Node usage percentage: %.2f", percentage)
}

func debugNodeMargin(margin float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	logger.Debugf("Node's margin: %.2f", margin)
}

func debugWeightsNMS(weights map[string]map[string]uint) {
	if !logging.GetDebugMode() {
		return
	}

	var buf strings.Builder
	buf.WriteString("Calculated weights:\n")
	for fun, nodesWeights := range weights {
		fmt.Fprintf(&buf, "  - function %s:\n", fun)
		for node, weight := range nodesWeights {
			fmt.Fprintf(&buf, "    - Node %s: %d\n", node, weight)
		}
	}
	logging.Logger().Debug(buf.String())
}
