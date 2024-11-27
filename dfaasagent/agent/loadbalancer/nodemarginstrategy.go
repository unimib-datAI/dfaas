package loadbalancer

import (
	"fmt"
	"time"
	"math"
	"encoding/json"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/bcicen/go-haproxy"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/communication"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/hacfgupd"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/forecaster"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/ofpromq"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/offuncs"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/hasock"
)

// In this file is implemented the Node Margin strategy

///////////////////// CONSTANTS ///////////////////////////

const cpuUsageNodeMetric = "cpu_usage_node"
const ramUsageNodeMetric = "ram_usage_node"
const powerUsageNodeMetric = "power_usage_node"

// Struct representing a NodeMarginStrategy instance, which implements the Strategy interface
type NodeMarginStrategy struct {
	hacfgupdater 	 hacfgupd.Updater
	nodestbl 		 *nodestbl.TableNMS
	ofpromqClient 	 ofpromq.Client
	offuncsClient 	 offuncs.Client
	hasockClient  	 haproxy.HAProxyClient
	forecasterClient forecaster.Client
	nodeInfo 		 nodeInfo
	// Functions groups
	funcsGroups 	 Groups
	// Map with a metric name as key
	// and max threshold values used to determine if the node is in overload
	maxValues 	     map[string]float64
	// Map of target nodes, with node ID of a common neighbour as key,
	// and array of common functions as value
	targetNodes 	 map[string][]string
	// Map containing for each node's function (key) another map containing
	// for each node the corresponding forwarding weight 
	weights 		 map[string]map[string]uint
}

// groupsLoad represents the invocation rates for each group of functions on a node
type GroupsLoad struct {
	RateHighUsage 	float64
	RateLowUsage 	float64
	RateMediumUsage float64
}

// Private struct containing info about us
type nodeInfo struct {
	nodeType 			int 				// Node type (heavy=0, mid=1, light=2)
	funcs 				[]string			// Our OpenFaaS functions
	funcsGroupsLoad 	GroupsLoad			// Load rates for each group of functions
	commonNeighboursNum int					// Number of neighbours with at least a function in common
	funcsRates 			map[string]float64  // Map with function name as key, and invocation rate as value
	margin 				float64				// Node's margin
	overload 			bool				// True if node is in overload
	metricsPredictions 	map[string]float64  // Map with metric name as key, and it's predicted value
}

//////////////////// PUBLIC FUNCTIONS FOR NODE MARGIN STRATEGY ////////////////////

// RunStrategy handles the periodic execution of the recalculation function. It
// should run in a goroutine
func (strategy *NodeMarginStrategy) RunStrategy() error {
	var millisNow, millisSleep int64
	var err error

	millisInterval := int64(_config.RecalcPeriod / time.Millisecond)

	strategy.maxValues[cpuUsageNodeMetric] = _config.CPUThresholdNMS
	strategy.maxValues[ramUsageNodeMetric] = _config.RAMThresholdNMS
	strategy.maxValues[powerUsageNodeMetric] = _config.PowerThresholdNMS

	strategy.nodeInfo.nodeType = _config.NodeType
	strategy.nodeInfo.overload = false

	var cpuUsage = make(map[string]float64)
	var ramUsage = make(map[string]float64)

	for {
		cpuUsage, err = strategy.ofpromqClient.QueryCPUusage(_config.RecalcPeriod)
		if err != nil {
			return errors.Wrap(err, "Error while executing Prometheus query")
		}
		debugPromCPUusage(_config.RecalcPeriod, cpuUsage)

		ramUsage, err = strategy.ofpromqClient.QueryRAMusage(_config.RecalcPeriod)
		if err != nil {
			return errors.Wrap(err, "Error while executing Prometheus query")
		}
		debugPromRAMusage(_config.RecalcPeriod, ramUsage)

		err = strategy.publishNodeInfo()
		if err != nil {
			return err
		}

		strategy.updateCommonNeighbours()

		strategy.nodeInfo.funcsRates, err = strategy.getFunctionsRates()

		strategy.funcsGroups, err = GetFuncsGroups()
		if err != nil {
			return err
		}

		strategy.nodeInfo.funcsGroupsLoad, err = strategy.getFuncsGroupsLoad()
		if err != nil {
			return err
		}

		strategy.nodeInfo.metricsPredictions, err = strategy.getNodeMetricPredictions(strategy.nodeInfo.nodeType, strategy.nodeInfo.funcsGroupsLoad)
		if err != nil {
			return err
		}

		strategy.nodeInfo.overload = strategy.isNodeOverloaded(strategy.nodeInfo.metricsPredictions)

		strategy.nodeInfo.margin = strategy.calculateMargin(strategy.maxValues)

		err = strategy.sendMarginToNeighbours()
		if err != nil {
			return err
		}

		strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
			debugNodesTblContentNMS(entries)
			return nil
		})

		strategy.weights, err = strategy.calculateWeights()
		if err != nil {
			return err
		}

		err = strategy.setProxyWeights()
		if err != nil {
			return err
		}

		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - (millisNow % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)
	}
}

// OnReceived should be executed every time a message from a peer is received
func (strategy *NodeMarginStrategy) OnReceived(msg *pubsub.Message) error {
	var msgForType struct{ MsgType string }
	err := json.Unmarshal(msg.GetData(), &msgForType)
	if err != nil {
		return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
	}

	switch msgForType.MsgType {
	case StrMsgNodeInfoTypeNMS:
		var objMsg MsgNodeInfoNMS
		err := json.Unmarshal(msg.GetData(), &objMsg)
		if err != nil {
			return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
		}

		strategy.processMsgNodeInfoNMS(msg.GetFrom().String(), &objMsg)
	case StrMsgNodeMarginInfoTypeNMS:
		var objMsg MsgNodeMarginInfoNMS
		err := json.Unmarshal(msg.GetData(), &objMsg)
		if err != nil {
			return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
		}

		strategy.processMsgNodeMarginInfoNMS(msg.GetFrom().String(), &objMsg)
	}

	return nil
}

//////////////////// PRIVATE FUNCTIONS FOR NODE MARGIN STRATEGY ////////////////////

// Send to other nodes our information (HAProxyHost, HAProxyPort, node type, functions list)
func (strategy *NodeMarginStrategy) publishNodeInfo() error {
	var err error

	// Obtain our function names list
	strategy.nodeInfo.funcs, err = strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return errors.Wrap(err, "Error while getting functions info from OpenFaaS")
	}

	msg := MsgNodeInfoNMS{
		MsgType:		StrMsgNodeInfoTypeNMS,
		HAProxyHost: 	_config.HAProxyHost,
		HAProxyPort:	_config.HAProxyPort,
		NodeType:		strategy.nodeInfo.nodeType,
		MaxValues:		strategy.maxValues,
		Functions:		strategy.nodeInfo.funcs,
	}
	debugMsgNodeInfoNMS(msg)

	err = communication.MarshAndPublish(msg)
	if err != nil {
		return err
	}

	return nil
}

// Check which neighbour has at least a function in common and update the relative information in nodestbl
func (strategy *NodeMarginStrategy) updateCommonNeighbours() {
	commonNeighbours := 0
	var targetNodes = make(map[string][]string)
	var common bool

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		for neighbourID, neighInfo := range entries {
			common = false
			var commonFuncs []string
			for i := 0; i < len(strategy.nodeInfo.funcs); i++ {
				for j := 0; j < len(neighInfo.Funcs); j++ {
					if strategy.nodeInfo.funcs[i] == neighInfo.Funcs[j] {
						commonFuncs = append(commonFuncs, strategy.nodeInfo.funcs[i])
						common = true
						break
					}
				}
			}
			if common {
				commonNeighbours += 1
				neighbour, _ := entries[neighbourID]
				neighbour.CommonNeighbour = true
				entries[neighbourID] = neighbour
				targetNodes[neighbourID] = commonFuncs
			}
		}

		// Update targetNodes
		strategy.targetNodes = targetNodes
		// Update commonNeighboursNum
		strategy.nodeInfo.commonNeighboursNum = commonNeighbours

		return nil
	})
}

// Obtain functions invocation rates of the last RecalcPeriod from HAProxy
func (strategy *NodeMarginStrategy) getFunctionsRates() (map[string]float64, error) {
	var functionRates = make(map[string]float64)

	for _, funcName := range strategy.nodeInfo.funcs {
		stName := fmt.Sprintf("st_users_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(&strategy.hasockClient, stName)

		if err != nil {
			return nil, err
		} else {
			for _, stEntry := range stContent {
				functionRates[funcName] = float64(stEntry.HTTPReqRate)
			}
		}
	}

	return functionRates, nil
}

// Calculate invocation rate for each group of functions
// NOTE: at the moment invocation rates on node are gathered from HAProxy. In future
// they should be gathered from the Forecaster, which predicts the future load received from the node
func (strategy *NodeMarginStrategy) getFuncsGroupsLoad() (GroupsLoad, error) {
		var err error
		
		strategy.nodeInfo.funcsRates, err = strategy.getFunctionsRates()
		if err != nil {
			return GroupsLoad{}, err
		}
	
		var nodeGroupsLoad GroupsLoad

		nodeGroupsLoad.RateHighUsage = 0.0
		nodeGroupsLoad.RateLowUsage = 0.0
		nodeGroupsLoad.RateMediumUsage = 0.0
		
		for funcName, rate := range strategy.nodeInfo.funcsRates {
			if contains(strategy.funcsGroups.HighUsage, funcName) {
				nodeGroupsLoad.RateHighUsage += rate
			} else if contains(strategy.funcsGroups.MediumUsage, funcName) {
				nodeGroupsLoad.RateMediumUsage += rate
			} else if contains(strategy.funcsGroups.LowUsage, funcName) {
				nodeGroupsLoad.RateLowUsage += rate
			}
		}

		debugFuncsLoad(nodeGroupsLoad)
	
		return nodeGroupsLoad, nil
}

// Get node metric predictions from Forecaster
func (strategy *NodeMarginStrategy) getNodeMetricPredictions(nodeType int, load GroupsLoad) (map[string]float64, error) {
	var err error
	
	var req forecaster.NodeMetricPredReq
	req.Node_type = nodeType
	req.Rate_group_HIGH_USAGE = load.RateHighUsage
	req.Rate_group_LOW_USAGE = load.RateLowUsage
	req.Rate_group_MEDIUM_USAGE = load.RateMediumUsage

	var resp forecaster.NodeMetricPredRes
	resp, err = strategy.forecasterClient.GetNodeUsagePredictions(req)
	if err != nil {
		return nil, errors.Wrap(err, "Error while executing request to Forecaster")
	}

	var predictions = make(map[string]float64)
	predictions[cpuUsageNodeMetric] = resp.Cpu_usage_node
	predictions[ramUsageNodeMetric] = resp.Ram_usage_node
	predictions[powerUsageNodeMetric] = resp.Power_usage_node

	debugNodeMetricPredictions(predictions)

	return predictions, nil
}

// Check if node is in overload (based on metrics predictions on node and maxValues)
func (strategy *NodeMarginStrategy) isNodeOverloaded(metricsPredictions map[string]float64) bool {
	overloaded := false
	for metric, value := range metricsPredictions {
		if value > strategy.maxValues[metric] {
			overloaded = true
			break
		}
	}

	debugOverloadNMS(overloaded)

	return overloaded
}

// Calculate the node usage percentage (based on metrics prediction on node)
func (strategy *NodeMarginStrategy) calculateNodeUsagePercentage(metricsPredictions map[string]float64, thresholds map[string]float64) float64 {
	var metricsPercentage = make(map[string]float64)
	var nodeUsagePercentage = 0.0

	for metric, _ := range metricsPredictions {
		metricsPercentage[metric] = (metricsPredictions[metric] * 100) / thresholds[metric]
	}

	for _, value := range metricsPercentage {
		nodeUsagePercentage += value
	}
	nodeUsagePercentage /= float64(len(metricsPercentage))

	debugNodeUsagePercentage(nodeUsagePercentage)

	return nodeUsagePercentage
}

// Calculate node's margin (based on information obtained with calculateNodeUsagePercentage function)
func (strategy *NodeMarginStrategy) calculateMargin(thresholds map[string]float64) float64 {
	var nodeUsagePercentage float64
	var margin = 0.0

	if strategy.nodeInfo.overload {
		return margin
	} else {
		nodeUsagePercentage = strategy.calculateNodeUsagePercentage(strategy.nodeInfo.metricsPredictions, thresholds)
		if strategy.nodeInfo.commonNeighboursNum > 0 {
			margin = (100 - nodeUsagePercentage) / float64(strategy.nodeInfo.commonNeighboursNum)
		}
	}

	debugNodeMargin(margin)

	return margin
}

// Send margin message to neighbours
func (strategy *NodeMarginStrategy) sendMarginToNeighbours() error {
	var err error
	
	// Publish margin and load info only if there are common neighbours
	if strategy.nodeInfo.commonNeighboursNum > 0 {
		var marginMsg MsgNodeMarginInfoNMS
		marginMsg.MsgType = StrMsgNodeMarginInfoTypeNMS
		marginMsg.Margin = strategy.nodeInfo.margin
		// Put expected load info in message only if margin is not zero
		if strategy.nodeInfo.margin > 0.0 {
			marginMsg.Load = strategy.nodeInfo.funcsGroupsLoad
		} else {
			marginMsg.Load = GroupsLoad{}
		}

		err = communication.MarshAndPublish(marginMsg)
		if err != nil {
			return err
		}
	}

	return nil
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

// Calculate forwarding weights for each function and neighbour
func (strategy *NodeMarginStrategy) calculateWeights() (map[string]map[string]uint, error) {
	logger := logging.Logger()

	var overload bool
	var myNodeID string
	var mainteined = make(map[string]float64)
	var mainteinedGroupsLoad GroupsLoad
	var fwdRequests = make(map[string]map[string]float64)
	var weights = make(map[string]map[string]uint)
	var iterator = make(map[string]int)

	overload = strategy.nodeInfo.overload
	myNodeID = _p2pHost.ID().String()

	// Copy nodeFuncsRates in mainteined map
	for fun, rate := range strategy.nodeInfo.funcsRates {
		mainteined[fun] = rate
	}

	// Copy funcsGroupsLoad in mainteinedGroupsLoad
	mainteinedGroupsLoad.RateHighUsage = strategy.nodeInfo.funcsGroupsLoad.RateHighUsage
	mainteinedGroupsLoad.RateMediumUsage = strategy.nodeInfo.funcsGroupsLoad.RateMediumUsage
	mainteinedGroupsLoad.RateLowUsage = strategy.nodeInfo.funcsGroupsLoad.RateLowUsage

	err := strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		// Init fwdRequests
		for neighID, _ := range entries {
			fwdRequests[neighID] = make(map[string]float64)
			for i := 0; i < len(strategy.nodeInfo.funcs); i++ {
				fwdRequests[neighID][strategy.nodeInfo.funcs[i]] = 0.0
			}
		}

		// Delete from targetNodes neighbours with margin=0 or without functions in common
		for targetID, targetFuncs := range strategy.targetNodes {
			if (entries[targetID].Margin == 0.0) || (len(targetFuncs) == 0) {
				delete(strategy.targetNodes, targetID)
			}
		}

		// Init iterator to select nodeTo and funcTo at each iteration
		iterator["targetIndex"] = -1
		for targetID, _ := range strategy.targetNodes {
			iterator[targetID] = 0
		}

		for (overload && (len(strategy.targetNodes) > 0)) {
			// Generate array of indexes to select different nodeTo at each iteration
			var targetKeys []string
			for targetID, _ := range strategy.targetNodes {
				targetKeys = append(targetKeys, targetID)
			}

			// Update nodes iterator
			iterator["targetIndex"] += 1
			if iterator["targetIndex"] >= len(targetKeys) {
				iterator["targetIndex"] = 0
			}

			nodeTo := targetKeys[iterator["targetIndex"]]
			funcTo := strategy.targetNodes[nodeTo][iterator[nodeTo]]

			logger.Debugf("Selected node: %s", nodeTo)
			logger.Debugf("Selected function: %s", funcTo)

			if mainteined[funcTo] > 0.0 {
				// Get nodeTo usage percentage with its original load
				var load GroupsLoad
				load.RateHighUsage = entries[nodeTo].Load.RateHighUsage
				load.RateMediumUsage = entries[nodeTo].Load.RateMediumUsage
				load.RateLowUsage = entries[nodeTo].Load.RateLowUsage

				logger.Debugf("Before requests forwarding (node %s):", nodeTo)
				nodeToPredictions, err := strategy.getNodeMetricPredictions(entries[nodeTo].NodeType, load)
				if err != nil {
					return err
				}
				nodeToPercentage := strategy.calculateNodeUsagePercentage(nodeToPredictions, entries[nodeTo].MaxValues)
				
				reqToTransfer := (mainteined[funcTo] * 0.01)

				// Update the rate group load in request to Forecaster corresponding to the group of funcTo
				var newLoad GroupsLoad
				newLoad.RateHighUsage = entries[nodeTo].Load.RateHighUsage
				newLoad.RateMediumUsage = entries[nodeTo].Load.RateMediumUsage
				newLoad.RateLowUsage = entries[nodeTo].Load.RateLowUsage
				if contains(strategy.funcsGroups.HighUsage, funcTo) {
					newLoad.RateHighUsage += reqToTransfer
				} else if contains(strategy.funcsGroups.MediumUsage, funcTo) {
					newLoad.RateMediumUsage += reqToTransfer
				} else if contains(strategy.funcsGroups.LowUsage, funcTo) {
					newLoad.RateLowUsage += reqToTransfer
				}

				for fun, rate := range fwdRequests[nodeTo] {
					if contains(strategy.funcsGroups.HighUsage, fun) {
						newLoad.RateHighUsage += rate
					} else if contains(strategy.funcsGroups.MediumUsage, fun) {
						newLoad.RateMediumUsage += rate
					} else if contains(strategy.funcsGroups.LowUsage, fun) {
						newLoad.RateLowUsage += rate
					}
				}
				logger.Debugf("After requests forwarding (node %s):", nodeTo)
				newNodeToPredictions, err := strategy.getNodeMetricPredictions(entries[nodeTo].NodeType, newLoad)
				if err != nil {
					return err
				}
				newNodeToPercentage := strategy.calculateNodeUsagePercentage(newNodeToPredictions, entries[nodeTo].MaxValues)

				margin := entries[nodeTo].Margin

				if margin > newNodeToPercentage - nodeToPercentage {
					mainteined[funcTo] -= reqToTransfer
					fwdRequests[nodeTo][funcTo] += reqToTransfer
					
					if contains(strategy.funcsGroups.HighUsage, funcTo) {
						mainteinedGroupsLoad.RateHighUsage -= reqToTransfer
					} else if contains(strategy.funcsGroups.MediumUsage, funcTo) {
						mainteinedGroupsLoad.RateMediumUsage -= reqToTransfer
					} else if contains(strategy.funcsGroups.LowUsage, funcTo) {
						mainteinedGroupsLoad.RateLowUsage -= reqToTransfer
					}
					
					logger.Debugf("This node's state after requests forwarding: ")
					nodePredictions, err := strategy.getNodeMetricPredictions(strategy.nodeInfo.nodeType, mainteinedGroupsLoad)
					if err != nil {
						return err
					}
					overload = strategy.isNodeOverloaded(nodePredictions)

					// Update nodeTo iterator
					iterator[nodeTo] += 1
					if iterator[nodeTo] >= len(strategy.targetNodes[nodeTo]) {
						iterator[nodeTo] = 0
					}
				} else {
					// Remove funcTo from possibilities of nodeTo
					var functions []string
					for i := 0; i < len(strategy.targetNodes[nodeTo]); i++ {
						if funcTo != strategy.targetNodes[nodeTo][i] {
							functions = append(functions, strategy.targetNodes[nodeTo][i])
						}
					}
					strategy.targetNodes[nodeTo] = functions

					// Reset functions iterator of nodeTo
					if iterator[nodeTo] >= len(strategy.targetNodes[nodeTo]) {
						iterator[nodeTo] = 0
					}

					// Delete nodeTo if has no more functions available
					if len(strategy.targetNodes[nodeTo]) == 0 {
						delete(strategy.targetNodes, nodeTo)
						// Reset target nodes iterator
						delete(iterator, nodeTo)
					}
				}
			} else {
				logger.Debugf("Selected function has no requests.")
				// Remove funcTo from all target nodes possibilities
				for targetID, targetFuncs := range strategy.targetNodes {
					for i := 0; i < len(targetFuncs); i++ {
						if targetFuncs[i] == funcTo {
							var functions []string
							for j := 0; j < len(targetFuncs); j++ {
								if funcTo != targetFuncs[j] {
									functions = append(functions, targetFuncs[j])
								}
							}
							strategy.targetNodes[targetID] = functions

							if iterator[targetID] >= len(strategy.targetNodes[targetID]) {
								iterator[targetID] = 0
							}

							// Delete nodeTo if has no more functions available
							if len(strategy.targetNodes[targetID]) == 0 {
								delete(strategy.targetNodes, targetID)
								// Reset target nodes iterator
								delete(iterator, targetID)
							}
							break
						}
					}
				}
			}
		}

		// Calculate weights
		for i := 0; i < len(strategy.nodeInfo.funcs); i++ {
			fun := strategy.nodeInfo.funcs[i]
			weights[fun] = make(map[string]uint)
			totalReq := strategy.nodeInfo.funcsRates[fun]
			// Weight for this node
			if totalReq == 0.0 {
				weights[fun][myNodeID] = constants.HAProxyMaxWeight
			} else {
				weights[fun][myNodeID] = uint(math.Round((mainteined[fun] / totalReq) * constants.HAProxyMaxWeight))
			}
			// Weights for neighbours
			for neighID, _ := range entries {
				if totalReq == 0.0 {
					weights[fun][neighID] = 0
				} else {
					weights[fun][neighID] = uint(math.Round((fwdRequests[neighID][fun] / totalReq) * constants.HAProxyMaxWeight))
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	debugWeightsNMS(weights)

	return weights, nil
}

// Update HAProxy configuration with new weights
func (strategy *NodeMarginStrategy) setProxyWeights() error {
	myID := _p2pHost.ID().String()

	var hacfg *HACfgNMS
	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		hacfg = strategy.createHACfgObject(
			myID,
			_config.OpenFaaSHost,
			_config.OpenFaaSPort,
			_config.HttpServerHost,
			_config.HttpServerPort,
			_config.RecalcPeriod,
			entries,
			strategy.weights,
		)
		return nil
	})

	err := strategy.updateHAProxyConfig(hacfg)
	if err != nil {
		return err
	}
	return nil
}

// processMsgNodeInfoNMS processes a node info message received from pubsub
func (strategy *NodeMarginStrategy) processMsgNodeInfoNMS(sender string, msg *MsgNodeInfoNMS) error {
	logger := logging.Logger()
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	if logging.GetDebugMode() {
		logger.Debugf("Received node info message from node %s", sender)
		logger.Debugf("Node %s type: %d", sender, msg.NodeType)
		logger.Debugf("Node %s HAProxyHost: %s", sender, msg.HAProxyHost)
		logger.Debugf("Node %s HAProxyPort: %d", sender, msg.HAProxyPort)
		logger.Debugf("Max Values: CPU=%f, RAM=%f, Power=%f",
				msg.MaxValues[cpuUsageNodeMetric], msg.MaxValues[ramUsageNodeMetric], msg.MaxValues[powerUsageNodeMetric])
		
		var funcs string
		for i := 0; i < len(msg.Functions); i++ {
			funcs += msg.Functions[i] + ", "
		}
		logger.Debugf("Node %s functions: %s", sender, funcs) 
	}

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		logger.Debugf("Setting received values for node %s into table", sender)
		_, present := entries[sender]
		if !present {
			entries[sender] = &nodestbl.EntryNMS{
				CommonNeighbour: 	false,
				Load:				nodestbl.Load{},
			}
			logger.Debugf("Node %s was not present and has been added to the table", sender)
		}
		entries[sender].TAlive = time.Now()
		entries[sender].HAProxyHost = msg.HAProxyHost
		entries[sender].HAProxyPort = msg.HAProxyPort
		entries[sender].NodeType = msg.NodeType
		entries[sender].MaxValues = msg.MaxValues
		entries[sender].Funcs = msg.Functions

		return nil
	})

	return nil
}

// processMsgNodeMarginInfoNMS processes a node margin info message received from pubsub
func (strategy *NodeMarginStrategy) processMsgNodeMarginInfoNMS(sender string, msg *MsgNodeMarginInfoNMS) error {
	logger := logging.Logger()
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	strategy.nodestbl.SafeExec(func(entries map[string]*nodestbl.EntryNMS) error {
		// Check if sender is a common neighbour. If not, ignore the message
		if entries[sender].CommonNeighbour {
			if logging.GetDebugMode() {
				logger.Debugf("Received margin info message from node %s", sender)
				logger.Debugf("Margin: %f", msg.Margin)
				logger.Debugf("Load: High Usage=%f, Low Usage=%f, Medium Usage=%f",
				msg.Load.RateHighUsage, msg.Load.RateLowUsage, msg.Load.RateMediumUsage)
			}
			logger.Debugf("Setting received values for node %s into table", sender)

				// Check if exists the neighbour with "sender" ID with its info in entries before adding new info to avoid errors
				_, exists := entries[sender]
				if exists {
					entries[sender].TAlive = time.Now()
					entries[sender].Margin = msg.Margin
					entries[sender].Load.RateHighUsage = msg.Load.RateHighUsage
					entries[sender].Load.RateMediumUsage = msg.Load.RateLowUsage
					entries[sender].Load.RateMediumUsage = msg.Load.RateLowUsage
				}
		} else {
			logger.Debugf("Ignore margin info message from node %s (not a common neighbour)", sender)
		}
		return nil
	})

	return nil
}

// updateHAProxyConfig updates the HAProxy config file with the provided object.
// Note: Go templates sort maps by key (see https://stackoverflow.com/questions/33860961/sorted-map-iteration-in-go-templates#comment76560806_33862260)
func (strategy *NodeMarginStrategy) updateHAProxyConfig(hacfg *HACfgNMS) error {
	hacfg.Now = time.Now()

	// hacfg is a struct of type HACfgNMS, that is use as content
	// for writing template file.
	// Indeed, in template file is possible to access fields
	// of this struct (see file of template .tmpl).
	// Template is referred to a data structure, in that case HACfgNMS type.
	// See https://golang.org/pkg/text/template/.
	return strategy.hacfgupdater.UpdateHAConfig(hacfg)
}

// Method which creates and returns the HACfgNMS object,
// used from method updateHAProxyConfig to update the HAProxy configuration
func (strategy *NodeMarginStrategy) createHACfgObject(
	myNodeID string,
	openFaaSHost string,
	openFaaSPort uint,
	httpServerHost string,
	httpServerPort uint,
	recalcPeriod time.Duration,
	entries map[string]*nodestbl.EntryNMS,
	funcsWeights map[string]map[string]uint,
) *HACfgNMS {
	hacfg := &HACfgNMS{
		HACfg: HACfg{ 
			MyNodeID: 	  myNodeID,
			HAProxyHost:  _config.HAProxyHost,
			OpenFaaSHost: openFaaSHost,
			OpenFaaSPort: openFaaSPort,
		},

		HttpServerHost: httpServerHost,
		HttpServerPort: httpServerPort,

		StrRecalc:  recalcPeriod.String(),

		Nodes:     map[string]*HACfgNodeNMS{},
		Functions: map[string]*HACfgFuncNMS{},
	}

	// For each node write Host and port.
	for nodeID, entry := range entries {
		hacfg.Nodes[nodeID] = &HACfgNodeNMS{
			HAProxyHost: entry.HAProxyHost,
			HAProxyPort: entry.HAProxyPort,
		}
	}

	// For each function write weights for load-balancing
	for funcName, weights := range funcsWeights {
		hacfg.Functions[funcName] = &HACfgFuncNMS{
			Weights: weights,
		}
	}

	// Weight are used in template file for load-balancing, with custom
	// weights toward other nodes of p2p net.
	// See: https://serverfault.com/questions/113637/haproxy-roundrobin-weights
	// Also doc: http://cbonte.github.io/haproxy-dconv/configuration-1.4.html#5-weight

	return hacfg
}
