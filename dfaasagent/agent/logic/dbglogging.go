package logic

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

func debugPromInvoc(timeSpan time.Duration, data map[string]float64) {
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
		logger.Debugf("  - FUNC %s: %.2f req/s", funcName, data[funcName])
	}
}

func debugPromServiceCount(data map[string]float64) {
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
		logger.Debugf("  - FUNC %s: %.0f active function replicas", funcName, data[funcName])
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

func debugNodesTblContent(entries map[string]*nodestbl.Entry) {
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
