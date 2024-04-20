package logic

import (
	"fmt"
	"time"
	"encoding/json"
	"os"
	"io/ioutil"

	"github.com/bcicen/go-haproxy"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/communication"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/hasock"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/p2phostutils"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/forecaster"
)

//////////////////// PRIVATE VARS FOR RECALC ////////////////////

// Private struct containing variables specific to the recalc algorithm, which
// need to be shared amongst the two recalc steps
var _recalc = struct {
	nodeIDs         []peer.ID                     // IDs of the connected p2p nodes
	stats           []*haproxy.Stat               // HAProxy stats
	funcs           map[string]uint               // Our OpenFaaS functions with dfaas.maxrate limits
	userRates       map[string]float64            // Invocation rates for users only (in req/s) (from HAProxy stick-tables)
	afet            map[string]float64            // Average Function Execution Times (from Prometheus)
	invoc           map[string]map[string]float64 // Invocation rates (in req/s) (from Prometheus)
	serviceCount    map[string]int
	cpuUsage        map[string]float64
	ramUsage        map[string]float64
	perFuncCpuUsage map[string]float64
	perFuncRamUsage map[string]float64

	// For each function, the value is true if the node is currently in overload
	// mode (req/s >= maxrate), false if underload
	overloads map[string]bool
}{}

// var expJson ExperimentJson // Struct that represent Json file
var it = 0 // Number of agent loop iterations

//////////////////// PUBLIC FUNCTIONS FOR RECALC ////////////////////

// RunRecalc handles the periodic execution of the recalculation function. It
// should run in a goroutine
func RunRecalc() error {
	var millisNow, millisSleep int64
	var err error

	millisInterval := int64(_config.RecalcPeriod / time.Millisecond)
	millisIntervalHalf := millisInterval / 2

	for {
		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - (millisNow % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)

		err = recalcStep1()
		if err != nil {
			return err
		}

		millisNow = time.Now().UnixNano() / 1000000
		millisSleep = millisInterval - ((millisNow + millisIntervalHalf) % millisInterval)
		time.Sleep(time.Duration(millisSleep) * time.Millisecond)

		err = recalcStep2()
		if err != nil {
			return err
		}
	}
}

//////////////////// PRIVATE FUNCTIONS FOR RECALC ////////////////////

func recalcStep1() error {
	var err error
	logger := logging.Logger()
	millisNow := time.Now().UnixNano() / 1000000
	logger.Debugf("#################### RECALC: STEP 1 (UnixMillis %d) ####################", millisNow)

	//////////////////// EXAMPLE TEXT MESSAGE ////////////////////

	//err := communication.MarshAndPublish(MsgText{
	//	MsgType: StrMsgTextType,
	//	Text:    "I'm alive!",
	//})
	//if err != nil {
	//	return err
	//}

	//////////////////// GET LIST OF CONNECTED NODES ////////////////////

	_recalc.nodeIDs = p2phostutils.GetConnNodeIDsUniq(_p2pHost)
	debugConnectedNodes(_recalc.nodeIDs)

	//////////////////// GATHER HAPROXY STATS ////////////////////

	//_recalc.stats, err = _hasockClient.Stats()
	//if err != nil {
	//	return errors.Wrap(err, "Error while gathering HAProxy stats from socket")
	//}
	//debugHAProxyStats(_recalc.stats)

	//////////////////// GATHER INFO ABOUT OPENFAAS FUNCTIONS ////////////////////

	_recalc.funcs, err = _offuncsClient.GetFuncs()
	if err != nil {
		return errors.Wrap(err, "Error while getting functions info from OpenFaaS")
	}
	debugFuncs(_recalc.funcs)

	//////////////////// GATHER INFO FROM HAPROXY STICKTABLES st_users_func_* ////////////////////

	_recalc.userRates = map[string]float64{}

	for funcName := range _recalc.funcs {
		stName := fmt.Sprintf("st_users_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(&_hasockClient, stName)

		if err != nil {
			errWrap := errors.Wrap(err, "Error while reading the stick-table \""+stName+"\" from the HAProxy socket")
			logger.Error(errWrap)
			logger.Warn("Not changing userRates for stick-table \"" + stName + "\" but this should be ok")
		} else {
			for _, stEntry := range stContent {
				// There should be only one line, with key "80", which is the port of the HAProxy frontend
				_recalc.userRates[funcName] = float64(stEntry.HTTPReqCnt) / float64(_config.RecalcPeriod/time.Second) * 2
				// Note: the whole formula is multiplied by two at the end because we know we restarted HAProxy at the end of recalcStep2
			}
		}

		debugStickTable(stName, stContent)
	}
	debugHAProxyUserRates(_recalc.userRates)

	//////////////////// [NEW] GATHER INFO FROM HAPROXY STICKTABLES st_local_func_* ////////////////////

	for funcName := range _recalc.funcs {
		stName := fmt.Sprintf("st_local_func_%s", funcName)
		stContent, err := hasock.ReadStickTable(&_hasockClient, stName)

		if err != nil {
			errWrap := errors.Wrap(err, "Error while reading the stick-table \""+stName+"\" from the HAProxy socket")
			logger.Error(errWrap)
			logger.Warn("Not changing local rates for stick-table \"" + stName + "\" but this should be ok")
		}

		debugStickTable(stName, stContent)
	}

	//////////////////// [NEW] GATHER INFO FOR STICKTABLES OF DATA FROM OTHER NODES ////////////////////
	/*
		for funcName := range _recalc.funcs {
			for _, nodeID := range _recalc.nodeIDs {
				stName := fmt.Sprintf("st_other_node_%s_%s", funcName, nodeID.String())
				stContent, err := hasock.ReadStickTable(&_hasockClient, stName)

				if err != nil {
					errWrap := errors.Wrap(err, "Error while reading the stick-table \""+stName+"\" from the HAProxy socket")
					logger.Error(errWrap)
					logger.Warn("Not changing other nodes rates for stick-table \"" + stName + "\" but this should be ok")
				}

				debugStickTable(stName, stContent)
			}
		}
	*/
	//////////////////// GATHER INFO FROM PROMETHEUS ////////////////////

	_recalc.afet, err = _ofpromqClient.QueryAFET(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while execting Prometheus query")
	}
	debugPromAFET(_config.RecalcPeriod, _recalc.afet)

	_recalc.invoc, err = _ofpromqClient.QueryInvoc(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromInvoc(_config.RecalcPeriod, _recalc.invoc)

	_recalc.serviceCount, err = _ofpromqClient.QueryServiceCount()
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromServiceCount(_recalc.serviceCount)

	_recalc.cpuUsage, err = _ofpromqClient.QueryCPUusage(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromCPUusage(_config.RecalcPeriod, _recalc.cpuUsage)

	_recalc.ramUsage, err = _ofpromqClient.QueryRAMusage(_config.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromRAMusage(_config.RecalcPeriod, _recalc.ramUsage)

	if len(_recalc.funcs) > 0 {
		// Get function's name as a slice.
		funcNames := make([]string, len(_recalc.funcs))
		i := 0
		for k := range _recalc.funcs {
			funcNames[i] = k
			i++
		}

		_recalc.perFuncCpuUsage, err = _ofpromqClient.QueryCPUusagePerFunction(_config.RecalcPeriod, funcNames)
		if err != nil {
			return errors.Wrap(err, "Error while executing Prometheus query")
		}
		debugPromCPUusagePerFunction(_config.RecalcPeriod, _recalc.perFuncCpuUsage)

		_recalc.perFuncRamUsage, err = _ofpromqClient.QueryRAMusagePerFunction(_config.RecalcPeriod, funcNames)
		if err != nil {
			return errors.Wrap(err, "Error while executing Prometheus query")
		}
		debugPromRAMusagePerFunction(_config.RecalcPeriod, _recalc.perFuncRamUsage)
	}

	///////// GET NODE USAGE PREDICTIONS FROM FORECASTER ///////////////
	var functionRates = make(map[string]float64)
	for funcName, item := range _recalc.invoc {
		functionRates[funcName] = 0
		for _, rate := range item {
			functionRates[funcName] += rate
		}
	}

	// Read functions groups from group_list.json
	_groupListFilePath := _config.GroupListFilePath
	jsonGroupsFile, err := os.Open(_groupListFilePath)
	if err != nil {
		return errors.Wrap(err, "Error while reading group list json file")
	}
	jsonGroups, err := ioutil.ReadAll(jsonGroupsFile)
	if err != nil {
		return errors.Wrap(err, "Error while converting group list json file into byte array")
	}
	defer jsonGroupsFile.Close()

	// Convert functions rates in groups rates and prepare json request
	var functionGroups forecaster.Groups
	json.Unmarshal(jsonGroups, &functionGroups)

	var request forecaster.PredictionRequest
	request.Node_type = _config.NodeType

	forecaster.SetRequestGroupsRates(functionRates, functionGroups, &request)
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "Error while constructing json request to Forecaster")
	}
	jsonBodyStr := string(jsonBody)
	logger.Debugf("Json request to Forecaster: %s", jsonBody)
	
	// send request to Forecaster
	var jsonResp string
	jsonResp, err = _forecasterClient.GetNodeUsagePredictions(jsonBodyStr)
	if err != nil {
		return errors.Wrap(err, "Error while executing request to Forecaster")
	}
	logger.Debugf("Forecaster Json response: %s", jsonResp)

	//////////////////// OVERLOAD / UNDERLOAD MODE DECISION ////////////////////

	_recalc.overloads = map[string]bool{}

	for funcName, maxRate := range _recalc.funcs {
		logger.Debugf("Computing if %s function is on overload", funcName)
		invocRate, present := _recalc.userRates[funcName]

		if !present || invocRate < float64(maxRate) {
			_recalc.overloads[funcName] = false
		} else {
			_recalc.overloads[funcName] = true
		}
	}
	debugOverloads(_recalc.overloads) // Debug purpose.

	it++

	//////////////////// LIMITS AND WEIGHTS CALCULATIONS ////////////////////

	for funcName, ovrld := range _recalc.overloads {
		logger.Debugf("Calculating limits and weights for %s function", funcName)
		if ovrld {
			// Set all funcData.LimitIn to zero for this function
			logger.Debugf("%s function is on overload! Setting LimitIn to 0", funcName)
			_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						funcData.LimitIn = 0
					}
				}

				return nil
			})
		} else {
			// Calculate the rate margin
			logger.Debugf("Calculating rate margin for %s function", funcName)
			invocRate, present := _recalc.userRates[funcName]
			maxRate := _recalc.funcs[funcName]
			logger.Debugf("%s function invocation rate is %f", funcName, invocRate)
			logger.Debugf("%s function max rate is %d", funcName, maxRate)
			var margin uint
			if present {
				margin = maxRate - uint(invocRate)
			} else {
				margin = maxRate
			}

			logger.Debugf("%s function margin equal to %d", funcName, margin)

			// Set all funcData.Weight to zero for this function, and set the
			// LimitIn for each node
			logger.Debugf("Setting Weight to 0 for %s function and setting LimitIn for each node", funcName)
			_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
				nNodes := uint(0)

				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						// Weights represent likelihood of send a request toward i-th
						// function instance.
						// Considering that this function instance is labelled as "underload"
						// it is not necessary to send request towards other nodes.
						funcData.NodeWeight = 0
						nNodes++
						logger.Debugf("Set Weight to 0 for %s function", funcName)
					}
				}

				// Note: if nNodes == 0, it means that (for now) i am the only
				// one to have this function, so i don't have to set the LimitIn
				// for anyone because no one needs it. Note also that the
				// nodestbl.SetReceivedValues() function sets the LimitIn to
				// zero, so not setting it here is ok

				if nNodes > 0 {
					limitIn := margin / nNodes // Equal distribution! May be
					// replaced in the future with a more efficient algorithm

					for _, entry := range entries {
						funcData, present := entry.FuncsData[funcName]
						if present {
							funcData.LimitIn = float64(limitIn)
							logger.Debugf("Set LiminIn to %f for %s function", funcData.LimitIn, funcName)
						}
					}
				}

				return nil
			})
		}
	}

	//////////////////// PRINT CONTENT OF NODESTBL ////////////////////

	_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
		debugNodesTblContent(entries)
		return nil
	})

	//////////////////// P2P MESSAGES PUBLISHING ////////////////////

	limits := map[string]map[string]float64{}

	_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
		for _, nodeID := range _recalc.nodeIDs {
			strNodeID := nodeID.String()

			entry, present := entries[strNodeID]
			if present {
				// If this node has sent me some messages before, i send him the
				// limits according to the nodestbl
				limits[strNodeID] = map[string]float64{}
				for funcName, funcData := range entry.FuncsData {
					limits[strNodeID][funcName] = funcData.LimitIn
				}
			} else {
				// If this node has not sent me anything before, i send him all
				// the functions i have, but with all limits set to zero
				limits[strNodeID] = map[string]float64{}
				for funcName := range _recalc.funcs {
					limits[strNodeID][funcName] = 0
				}
			}
		}

		return nil
	})

	msg := MsgNodeInfo{
		MsgType:     StrMsgNodeInfoType,
		HAProxyHost: _config.HAProxyHost,
		HAProxyPort: _config.HAProxyPort,
		FuncLimits:  limits,
	}
	debugMsgNodeInfo(msg)

	err = communication.MarshAndPublish(msg)
	if err != nil {
		return err
	}
	//////////////////// IF EVERYTHING OK, RETURN NIL ////////////////////

	return nil
}

func recalcStep2() error {
	var err error
	logger := logging.Logger()
	millisNow := time.Now().UnixNano() / 1000000
	logger.Debugf("#################### RECALC: STEP 2 (UnixMillis %d) ####################", millisNow)

	//////////////////// CALC WEIGHTS FOR FUNCTIONS IN OVERLOAD MODE ////////////////////

	for funcName, ovrld := range _recalc.overloads {
		if ovrld {
			// Calculate the weights for this function
			_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
				totLimitsOut := float64(0)

				// Loop on all node in _nodestbl, check if that node
				// has this function running; if is present sum the amount of
				// req/sec forwardable to this node.
				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						totLimitsOut += funcData.LimitOut
					}
				}

				if totLimitsOut <= 0 {
					// If no node is available to help me with this function, i
					// set totLimitsOut to 1, only to avoid division by zero
					// problems. All the weights will be zero anyway
					totLimitsOut = 1
				}

				// Loop on all all node in _nodestbl, if function funcName is present in this node
				// that is in "oveload" state, is present also in i-th node, calculate
				// weight for the instance of function in i-th node.
				// Weight is based on LimitOut (number of req/sec forwardable to this node)
				// divided by total forwardable request.
				// All multiplied by 100, that is the sum of weights; this op allow to
				// express weights as the percentage of requests forwarded by this node to
				// other functions that runs on other nodes.
				for _, entry := range entries {
					funcData, present := entry.FuncsData[funcName]
					if present {
						funcData.NodeWeight = uint(funcData.LimitOut * constants.HAProxyMaxWeight / totLimitsOut)
					}
				}

				return nil
			})
		}
	}

	//////////////////// PRINT CONTENT OF NODESTBL ////////////////////

	_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
		debugNodesTblContent(entries)
		return nil
	})

	//////////////////// UPDATE HAPROXY CONFIGURATION ////////////////////

	strMyself := _p2pHost.ID().String()

	var hacfg *HACfg
	_nodestbl.SafeExec(func(entries map[string]*nodestbl.Entry) error {
		hacfg = createHACfgObject(
			strMyself,
			_config.OpenFaaSHost,
			_config.OpenFaaSPort,
			_config.HttpServerHost,
			_config.HttpServerPort,
			_config.RecalcPeriod,
			entries,
			_recalc.funcs,
		)
		return nil
	})

	err = updateHAProxyConfig(hacfg)
	if err != nil {
		return err
	}

	//////////////////// IF EVERYTHING OK, RETURN NIL ////////////////////

	return nil
}
