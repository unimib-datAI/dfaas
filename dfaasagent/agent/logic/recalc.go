package logic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/bcicen/go-haproxy"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/communication"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/hasock"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/p2phostutils"
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
var it = 0                 // Number of agent loop iterations

//////////////////// PUBLIC FUNCTIONS FOR RECALC ////////////////////

// RunRecalc handles the periodic execution of the recalculation function. It
// should run in a goroutine
func RunRecalc() error {
	var millisNow, millisSleep int64
	var err error

	millisInterval := int64(_flags.RecalcPeriod / time.Millisecond)
	millisIntervalHalf := millisInterval / 2

    /*
	//////////////////// [NEW] READ INIT FUNCTIONS JSON FILE /////////////////////////
	// The file is read only the first time that agent start

	// Open our jsonFile
	jsonFile, err := os.Open(_flags.InitFunctionsFile)

	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	} else {
	    fmt.Println("Successfully opened init functions file")
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	// read our opened jsonFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	json.Unmarshal(byteValue, &expJson)
	if err != nil {
		fmt.Println("Error while deserializing a JSON of init functions from file")
	}

	// Debug json read
	j, _ := json.MarshalIndent(expJson, "", "  ")
	fmt.Println(string(j))

	expJson.Outputs = []Output{}
	*/

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
				_recalc.userRates[funcName] = float64(stEntry.HTTPReqCnt) / float64(_flags.RecalcPeriod/time.Second) * 2
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

	_recalc.afet, err = _ofpromqClient.QueryAFET(_flags.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while execting Prometheus query")
	}
	debugPromAFET(_flags.RecalcPeriod, _recalc.afet)

	_recalc.invoc, err = _ofpromqClient.QueryInvoc(_flags.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromInvoc(_flags.RecalcPeriod, _recalc.invoc)

	_recalc.serviceCount, err = _ofpromqClient.QueryServiceCount()
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromServiceCount(_recalc.serviceCount)

	_recalc.cpuUsage, err = _ofpromqClient.QueryCPUusage(_flags.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromCPUusage(_flags.RecalcPeriod, _recalc.cpuUsage)

	_recalc.ramUsage, err = _ofpromqClient.QueryRAMusage(_flags.RecalcPeriod)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromRAMusage(_flags.RecalcPeriod, _recalc.ramUsage)

	// Get function's name as a slice.
	funcNames := make([]string, len(_recalc.funcs))
	i := 0
	for k := range _recalc.funcs {
		funcNames[i] = k
		i++
	}

	_recalc.perFuncCpuUsage, err = _ofpromqClient.QueryCPUusagePerFunction(_flags.RecalcPeriod, funcNames)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromCPUusagePerFunction(_flags.RecalcPeriod, _recalc.perFuncCpuUsage)

	_recalc.perFuncRamUsage, err = _ofpromqClient.QueryRAMusagePerFunction(_flags.RecalcPeriod, funcNames)
	if err != nil {
		return errors.Wrap(err, "Error while executing Prometheus query")
	}
	debugPromRAMusagePerFunction(_flags.RecalcPeriod, _recalc.perFuncRamUsage) // Note: timeSpan not used.

	//////////////////// OVERLOAD / UNDERLOAD MODE DECISION ////////////////////

	_recalc.overloads = map[string]bool{}

	for funcName, maxRate := range _recalc.funcs {
		invocRate, present := _recalc.userRates[funcName]

		if !present || invocRate < float64(maxRate) {
			_recalc.overloads[funcName] = false
		} else {
			_recalc.overloads[funcName] = true
		}
	}
	debugOverloads(_recalc.overloads) // Debug purpose.

    /*

	///////////////////// [NEW] EXTEND EXPERIMENT JSON FILE WITH METRICS //////////////////////////

	// Create slice of Functions type
	functionsSlice := []Function{}

	for _, funcName := range funcNames {
		name := funcName                               // Func name
		serviceCount := _recalc.serviceCount[funcName] // Service count

		invocRate, present := _recalc.userRates[funcName] // Invoc rate (users only) -- from HAproxy
		maxRate := _recalc.funcs[funcName]                // Max Rate
		var margin uint
		margin = 0

		if !_recalc.overloads[funcName] {
			if present {
				margin = maxRate - uint(invocRate)
			} else {
				margin = maxRate
			}
		}

		// Invocation rate (returned by gateway (prometheus) -- real), sum on different status codes
		irate := 0.0
		for _, rate := range _recalc.invoc[funcName] { // loop on (code, rate)
			irate += rate
		}

		// Afet
		afet := 0.001
		if !math.IsNaN(_recalc.afet[funcName]) {
			afet = _recalc.afet[funcName]
		}

		ram_xfunc := _recalc.perFuncRamUsage[funcName] // Ram x func
		cpu_xfunc := _recalc.perFuncCpuUsage[funcName] // CPU x func

		// State
		state := ""
		if _recalc.overloads[funcName] {
			state = "Overload"
		} else {
			state = "Underload"
		}

		f := Function{
			Name:          name,
			ServiceCount:  serviceCount,
			Margin:        margin,
			InvocRate:     uint(invocRate), //irate,
			Afet:          afet,
			RamxFunc:      ram_xfunc,
			CpuxFunc:      cpu_xfunc,
			MaxRate:       maxRate,
			State:         state,
			PromInvocRate: irate,
		}

		functionsSlice = append(functionsSlice, f)
	}

	// TODO: check if it works correctly
	timestamp := time.Now().Format("2006/01/02-15:04:05")

	out := Output{
		Timestamp: timestamp,
		RamUsage:  _recalc.ramUsage["localhost:9100"],
		CpuUsage:  _recalc.cpuUsage["localhost:9100"],
		Functions: functionsSlice,
	}

	expJson.Outputs = append(expJson.Outputs, out)

	jsonEncoding, err := json.MarshalIndent(expJson, "", " ")
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = ioutil.WriteFile("exp"+strconv.Itoa(it)+".json", jsonEncoding, 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}
	*/

	it++

	//////////////////// LIMITS AND WEIGHTS CALCULATIONS ////////////////////

	for funcName, ovrld := range _recalc.overloads {
		if ovrld {
			// Set all funcData.LimitIn to zero for this function
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
			invocRate, present := _recalc.userRates[funcName]
			maxRate := _recalc.funcs[funcName]
			var margin uint
			if present {
				margin = maxRate - uint(invocRate)
			} else {
				margin = maxRate
			}

			// Set all funcData.Weight to zero for this function, and set the
			// LimitIn for each node
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
		HAProxyHost: _flags.HAProxyHost,
		HAProxyPort: _flags.HAProxyPort,
		FuncLimits:  limits,
	}

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
			_flags.OpenFaaSHost,
			_flags.OpenFaaSPort,
			_flags.RecalcPeriod,
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
