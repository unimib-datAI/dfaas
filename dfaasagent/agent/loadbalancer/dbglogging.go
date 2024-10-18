package loadbalancer

import (
	"sort"
	"strings"
	"time"

	"github.com/bcicen/go-haproxy"
	"github.com/libp2p/go-libp2p-core/peer"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/hasock"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
)

//////////////////// PRIVATE FUNCTIONS FOR LOGGING ////////////////////

func debugConnectedNodes(nodeIDs []peer.ID) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	var b strings.Builder

	b.WriteString("Currently connected nodes:")

	if len(nodeIDs) == 0 {
		b.WriteString(" (none)")
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

	logger.Debug(b.String())
}

func debugHAProxyStats(stats []*haproxy.Stat) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	logger.Debug("HAProxy stats:")
	for _, item := range stats {
		logger.Debugf("  - %s (%s): %s (%d req/s)", item.PxName, item.SvName, item.Status, item.ReqRate)
	}
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

	logger.Debug("Average functions exec times (over " + timeSpan.String() + " time span):")
	for _, funcName := range keys {
		logger.Debugf("  - FUNC %s: %.3f ms", funcName, data[funcName]*1000)
	}
}

func debugPromInvoc(timeSpan time.Duration, data map[string]map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	logger.Debug("Functions invocation counts (over " + timeSpan.String() + " time span):")
	for _, funcName := range keys {
		for code, rate := range data[funcName] {
			logger.Debugf("  - FUNC %s, CODE %s: %.2f req/s", funcName, code, rate)
		}
	}
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

	logger.Debug("Functions service counts:")
	for _, funcName := range keys {
		logger.Debugf("  - FUNC %s: %d active function replicas", funcName, data[funcName])
	}
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

	logger.Debug("Nodes CPU usage (over " + timeSpan.String() + " time span):")
	for _, instance := range keys {
		logger.Debugf("  - Instance %s CPU utilization: %.2f%%", instance, data[instance]*100)
	}
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

	logger.Debug("Nodes RAM usage (over " + timeSpan.String() + " time span):")
	for _, instance := range keys {
		logger.Debugf("  - Instance %s RAM utilization: %.2f%%", instance, data[instance]*100)
	}
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

	logger.Debug("Nodes CPU usage for function (over " + timeSpan.String() + " time span) and averaged on number of container:")
	for _, funcName := range keys {
		logger.Debugf("  - FUNC %s CPU utilization: %.2f%%", funcName, data[funcName]*100)
	}
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

	logger.Debug("Nodes RAM usage for function (over " + timeSpan.String() + " time span) and averaged on number of container:")
	for _, funcName := range keys {
		logger.Debugf("  - FUNC %s RAM utilization: %.2f%%", funcName, data[funcName]*100)
	}
}

func debugHAProxyUserRates(data map[string]float64) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	logger.Debug("Invocation rates of requests from users only (calculated from HAProxy stick-table):")
	for _, funcName := range keys {
		logger.Debugf("  - FUNC %s: %.2f req/s", funcName, data[funcName])
	}
}

func debugFuncs(data map[string]uint) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	logger.Debug("Available functions:")
	for _, funcName := range keys {
		logger.Debugf("  - FUNC %s: limit %d req/s", funcName, data[funcName])
	}
}

func debugOverloads(data map[string]bool) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var strMode string
	logger.Debug("Functions overload/underload modes:")
	for _, funcName := range keys {
		if data[funcName] {
			strMode = "Overload"
		} else {
			strMode = "Underload"
		}

		logger.Debugf("  - FUNC %s: %s", funcName, strMode)
	}
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

	logger.Debug("Content of nodestbl:")
	for _, nodeID := range nodeIDs {
		entry := entries[nodeID]

		logger.Debugf("  - NODE %s (HAProxy=%s:%d)",
			nodeID,
			entry.HAProxyHost,
			entry.HAProxyPort)

		funcNames := make([]string, 0, len(entry.FuncsData))
		for k := range entry.FuncsData {
			funcNames = append(funcNames, k)
		}

		sort.Strings(funcNames)

		for _, funcName := range funcNames {
			funcData := entry.FuncsData[funcName]

			logger.Debugf("    - FUNC %s: LimitIn=%.2f LimitOut=%.2f NodeWeight=%d",
				funcName,
				funcData.LimitIn,
				funcData.LimitOut,
				funcData.NodeWeight)
		}
	}
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

	logger.Debug("Content of nodestbl:")
	for _, nodeID := range nodeIDs {
		entry := entries[nodeID]

		logger.Debugf("  - NODE %s (HAProxy=%s:%d), type: %d, common neighbour: %t, margin: %.2f",
			nodeID,
			entry.HAProxyHost,
			entry.HAProxyPort,
			entry.NodeType,
			entry.CommonNeighbour,
			entry.Margin)

		logger.Debugf("    - LOAD: highUsage=%.2f req/s, mediumhUsage=%.2f req/s, lowUsage=%.2f req/s",
						entry.Load.RateHighUsage, entry.Load.RateMediumUsage, entry.Load.RateLowUsage)

		logger.Debugf("    - FUNCTIONS:")
		for _, funcName := range entry.Funcs {
			logger.Debugf("       FUNC: %s", funcName)
		}
	}
}

func debugStickTable(stName string, stContent map[string]*hasock.STEntry) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	clients := make([]string, 0, len(stContent))
	for k := range stContent {
		clients = append(clients, k)
	}

	sort.Strings(clients)

	logger.Debug("Stick-table \"" + stName + "\" content:")
	for _, key := range clients {
		stEntry := stContent[key]
		logger.Debugf("  - key=%s: cnt=%d rate=%d", key, stEntry.HTTPReqCnt, stEntry.HTTPReqRate)
	}
}

func debugMsgNodeInfoRecalc(msg MsgNodeInfoRecalc) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	for _nodeID, _limits := range msg.FuncLimits {
		logger.Debugf("Functions limits for node %s:", _nodeID)
		for funcName := range _limits {
			logger.Debugf("	Function %s LimitOut: %f", funcName, _limits[funcName])
		}
	}
}

func debugMsgNodeInfoNMS(msg MsgNodeInfoNMS) {
	if !logging.GetDebugMode() {
		return
	}

	logger := logging.Logger()

	logger.Debugf("    - Node Type: %d", msg.NodeType)
	logger.Debugf("    - Node's functions: ")
	for i := 0; i < len(msg.Functions); i++ {
        logger.Debugf("       - " + msg.Functions[i] + ", ") 
    }
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

	logger.Debugf("Node's usage predictions:")
	logger.Debugf("Cpu usage: %.2f, Ram usage: %.2f, Power usage: %.2f", 
					predictions[cpuUsageNodeMetric],
					predictions[ramUsageNodeMetric],
					predictions[powerUsageNodeMetric])
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

	logger := logging.Logger()

	logger.Debugf("Calculated weights:")
	for fun, nodesWeights := range weights {
		logger.Debugf("- " + fun + ": ")
		for node, weight := range nodesWeights {
			logger.Debugf("    - Node " + node + ": " + "%d", weight)
		}
	}
}