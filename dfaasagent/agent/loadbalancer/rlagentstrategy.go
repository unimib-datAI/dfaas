// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/hacfgupd"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/httpserver"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/hasock"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/promq"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/nodestbl"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/proxy"
)

type RLAgentStrategy struct {
	hacfgupdater  *hacfgupd.Updater
	runtimeapi    *proxy.RuntimeAPI
	nodestbl      *nodestbl.TableNMS
	offuncsClient *offuncs.Client
	promq         *promq.Client
	httpClient    *http.Client

	// Connection information for the RL model Web service. Used in RL Agent
	// phase.
	rlModelHost string
	rlModelPort uint

	// The RL Agent strategy consists of two cycling phases.
	// allLocalPhaseTimestamp and rlAgentPhaseTimestamp store the timestamps
	// indicating when each phase starts. They may be zero if not yet set,
	// typically at the beginning of the strategy.
	allLocalPhaseTimestamp time.Time
	rlAgentPhaseTimestamp  time.Time

	// targetFunction stores the function's name used in the RL Agent strategy
	// (the strategy currently supports only one deployed function)
	targetFunction string
}

// strategyPhase represents the two possibile phases of the RL Agent strategy.
type strategyPhase int

const (
	allLocalPhase strategyPhase = iota
	rlAgentPhase
)

func (p strategyPhase) String() string {
	switch p {
	case allLocalPhase:
		return "allLocalPhase"
	case rlAgentPhase:
		return "rlAgentPhase"
	default:
		return "Unknown"
	}
}

// strategyPhaseFromStage returns the current RL Agent strategy phase based on
// the given k6 stage index.
func strategyPhaseFromStage(stage int) (strategyPhase, error) {
	if stage < 0 {
		return allLocalPhase, errors.New("found negative stage, expected positive or zero")
	}

	switch stage % 4 {
	case 0, 1:
		return allLocalPhase, nil
	case 2, 3:
		return rlAgentPhase, nil
	default:
		return allLocalPhase, nil // Unreachable.
	}
}

const rlModelLogPath = "rl_model.log"

// RunStrategy handles the execution of the strategy. It is ran in a goroutine.
func (strategy *RLAgentStrategy) RunStrategy() error {
	logger := logging.Logger()

	// FIXME: We need to wait for all other neighbors to connect before doing
	// the setup, because once the HAProxy configuration is pushed we never
	// reconfigure HAProxy, only update the weights. Dynamic server
	// adding/remove is not supported currently.
	logger.Info("Waiting 1 minute before RL Agent strategy starts...")
	time.Sleep(1 * time.Minute)

	logger.Info("Starting RL Agent strategy...")

	logger.Info("Initial set-up: updating proxy with the discovered functions")
	if err := strategy.setup(); err != nil {
		return fmt.Errorf("failed to do initial set-up: %w", err)
	}
	// We start with a nil phase, since we do not detected any stage.
	var previousPhase *strategyPhase

	// We also keep track of the previous stage, but this information is only
	// used for debugging purposes.
	previousStage := -1

	// Wait some seconds to let HAProxy update tables, frontend and backends,
	// otherwise the StickTableField call may fail sometimes.
	time.Sleep(5 * time.Second)
	logger.Info("Strategy started")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		start := time.Now().UTC()

		// Get the "General Purpose Table 0" (gpt0) field for the key "global"
		// in the stick-table "main" (a frontend) from HAProxy. This field
		// represents the current "Stage" of k6, which HAProxy stores by parsing
		// the "DFaaS-K6-Stage" header from incoming requests.
		//
		// Note the value is always added with +1, so if it is 0 means no header
		// has been found.
		stage, err := hasock.StickTableField(strategy.httpClient, "main", "gpt0")
		if err != nil {
			if errors.Is(err, hasock.ErrEmpty) {
				// Disable log to avoid too many useless logs (one for each second).
				//logger.Warn("Cannot detect current stage: empty gpt0 field for stick-table main. Skipping iteration")
				continue
			}
			return fmt.Errorf("reading gpt0 field in stick-table main from HAProxy: %w", err)
		}
		if stage == 0 {
			logger.Warn("Cannot detect current stage: requests do not have DFaaS-K6-Stage header. Skipping iteration")
			continue
		} else {
			// We must subtract 1 because the HAProxy config always add +1 to
			// the stage counter to be able to represent the value 0 as "no
			// stage header found".
			stage--
		}
		logger.Infof("Current detected stage: %d", stage)

		if previousStage != stage {
			// We initialize the debug file every time the stage restart from 0.
			if stage == 0 {
				if err := debugRLModelToFileInit(rlModelLogPath); err != nil {
					return fmt.Errorf("failed to init log file: %w", err)
				}
			}

			// Also make sure to update previousStage!
			previousStage = stage
		}

		// From the current stage, we determine the strategy phase. The RL agent
		// operates in two phases:
		//
		//  1) In the first phase, the DFaaS node processes all incoming
		//  requests locally.
		//
		//  2) In the second phase, the DFaaS node uses statistics collected
		//  from both the first phase and previous second phase to query the RL
		//  model for an action, then applies that action (which may involve
		//  rejecting requests or forwarding them to other nodes).
		//
		// Since k6 is used for load testing, each stage consists of two
		// internal substages: stages 0 and 1 correspond to the "all-local"
		// phase, while stages 2 and 3 correspond to the RL-driven phase. This
		// pattern then repeats. Therefore, we can determine the current phase
		// by applying a modulo operation on the stage index.
		currentPhase, err := strategyPhaseFromStage(stage)
		if err != nil {
			logger.Warnf("Failed converting detected stage to strategy phase, skipping iteration: %w", err)
			continue
		}

		if previousPhase == nil {
			if currentPhase == allLocalPhase {
				logger.Infof("Starting strategy in %q phase", currentPhase)
			} else {
				logger.Warn("Previous phase is nil, current is not allLocalPhase. Skipping iteration")
				continue
			}
		} else if *previousPhase == currentPhase {
			logger.Infof("Current phase is equal to previous (%s). Nothing to do.", currentPhase)
			continue
		} else {
			logger.Infof("Moving from %q to %q phase", *previousPhase, currentPhase)
		}

		switch currentPhase {
		case allLocalPhase:
			err = strategy.allLocalPhase()
		case rlAgentPhase:
			err = strategy.rlAgentPhase()
		default:
			return fmt.Errorf("invalid phase %q", currentPhase)
		}
		if err != nil {
			return fmt.Errorf("running phase %q: %w", currentPhase, err)
		}

		previousPhase = &currentPhase

		duration := time.Since(start)
		httpserver.StrategyIterationDuration.Set(duration.Seconds())
		httpserver.StrategySuccessIterations.Inc()
		logger.Infof("Iteration completed. Duration: %s", duration.String())
	}

	return nil
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *RLAgentStrategy) OnReceived(msg *pubsub.Message) error {
	return nil
}

// setup runs the initial setup of the RL Agent strategy.
func (strategy *RLAgentStrategy) setup() error {
	logger := logging.Logger()

	// Strategy currently supports only one deployed function.
	funcs, err := strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return fmt.Errorf("failed to check deployed functions: %w")
	}
	if len(funcs) == 0 || len(funcs) > 1 {
		logger.Errorf("Strategy requires exactly one deployed function, found %d functions", len(funcs))
		return fmt.Errorf("exactly one function required, found %d", len(funcs))
	}
	strategy.targetFunction = funcs[0]
	logger.Infof("Target function of RL Agent strategy is %q", strategy.targetFunction)

	// We first set the initial proxy configuration. The allLocalPhase and
	// rlAgentPhase will update the weights based on this configuration.
	if err := strategy.initProxyConfig(); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	// We also set-up the internal http.Client used to contact the Data Plane
	// API when asking for the current stage. This allows used to reduce
	// connection overhead by reusing the same HTTP client.
	strategy.httpClient = &http.Client{
		Timeout: 900 * time.Millisecond,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return nil
}

// allLocalPhase runs the phase where the DFaaS agent processes all incoming
// requests locally.
func (strategy *RLAgentStrategy) allLocalPhase() error {
	if err := strategy.setAllLocal(); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	strategy.allLocalPhaseTimestamp = time.Now().UTC()

	return nil
}

// rlAgentPhase runs the phase where the DFaaS agent builds the observation for
// the RL model, queries the model, and applies the resulting action to the
// proxy.
func (strategy *RLAgentStrategy) rlAgentPhase() error {
	obs, err := strategy.buildObservation()
	if err != nil {
		return fmt.Errorf("building observation for RL phase: %w", err)
	}

	action, err := strategy.queryRLModel(obs)
	if err != nil {
		return fmt.Errorf("querying RL model for RL phase: %w", err)
	}

	if err := strategy.applyAction(action); err != nil {
		return fmt.Errorf("applying RL action for RL phase: %w", err)
	}

	strategy.rlAgentPhaseTimestamp = time.Now().UTC()

	return nil
}

// setAllLocal configures the proxy weights so that all incoming requests for
// each deployed function are processed locally.
func (strategy *RLAgentStrategy) setAllLocal() error {
	logger := logging.Logger()

	// We need the list of deployed functions becase we set the local processing
	// action function by function.
	functions, err := strategy.offuncsClient.GetFuncsNames()
	if err != nil {
		return fmt.Errorf("getting function names: %w", err)
	}

	// We need the list of neighbors for later use.
	neighbors := []string{}
	for _, peer := range _p2pHost.Network().Peers() {
		// "node_ID" is the required format.
		peerID := fmt.Sprintf("node_%s", peer)

		neighbors = append(neighbors, peerID)
	}

	// First set all actions, then apply them. The first key is the function
	// name/backend, the second level is the server action.
	weights := make(map[string]map[string]uint)

	// For each function we process all incoming requests locally by setting
	// weight 100 to "openfaas-local" server and weight 0 to "rejector" (used to
	// reject requests) and all other neighbors.
	for _, function := range functions {
		// For each function there are two backends: one handling incoming
		// forwarded requests and one handling incoming requests from clients.
		//
		// The agent currently controls only the incoming client requests
		// backend.
		backend := fmt.Sprintf("function_%s", function)
		weights[backend] = make(map[string]uint)

		weights[backend]["openfaas-local"] = 100
		weights[backend]["rejector"] = 0
		for _, neighborID := range neighbors {
			weights[backend][neighborID] = 0
		}
	}

	// Apply all weights.
	for backend, actions := range weights {
		for target, weight := range actions {
			if err := strategy.runtimeapi.SetWeight(backend, target, weight); err != nil {
				return fmt.Errorf("failed to set weight for %s on backend %s: %w", target, backend, err)
			}
		}
	}
	logger.Debugf("HAProxy updated with the following weights: %v", weights)

	return nil
}

// initProxyConfig initializes the proxy configuration via the Data Plane API.
//
// After this call, the proxy is managed exclusively through the Runtime API,
// where updates are performed only by changing the routing weights.
func (strategy *RLAgentStrategy) initProxyConfig() error {
	// Build the funcs map.
	funcs, err := strategy.offuncsClient.GetFuncsWithTimeout()
	if err != nil {
		return fmt.Errorf("get function metadata: %w", err)
	}

	// Add 1 seconds to base timeout (if given) to all functions.
	for _, timeout := range funcs {
		if timeout != nil {
			*timeout += 1000
		}
	}

	// Build neighbors info.
	neighbors := []string{}                  // Node's ID (node_XXX).
	neighborsPort := make(map[string]string) // Node's ID -> Node's host addr.
	neighborsHost := make(map[string]string) // Node's ID -> Node's port numb.

	for _, peer := range _p2pHost.Network().Peers() {
		host, err := extractSingleIPv4(_p2pHost, peer)
		if err != nil {
			return fmt.Errorf("failed to build neighbors information: %w", err)
		}

		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)

		neighbors = append(neighbors, peerID)
		neighborsHost[peerID] = host
		// FIXME: The remote proxy port may be different from local proxy port!
		neighborsPort[peerID] = strconv.FormatUint(uint64(_config.HAProxyPort), 10)
	}

	// Define and populate this anonymous struct to share data to the Go
	// template that will build the proxy configuration. See
	// haproxycfgrlagent.tml file.
	data := struct {
		Now           string
		DFaaSNodeID   string
		Functions     map[string]*uint
		Neighbors     []string
		NeighborsPort map[string]string
		NeighborsHost map[string]string
		OpenFaaSHost  string
		OpenFaaSPort  uint
		RejectorHost  string
		RejectorPort  uint
	}{
		Now:           time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:   _p2pHost.ID().String(),
		Functions:     funcs,
		Neighbors:     neighbors,
		NeighborsHost: neighborsHost,
		NeighborsPort: neighborsPort,
		OpenFaaSHost:  _config.OpenFaaSHost,
		OpenFaaSPort:  _config.OpenFaaSPort,
		RejectorHost:  _config.RejectorHost,
		RejectorPort:  _config.RejectorPort,
	}

	// The data structure will be passed to the hacfgupdater module and a new
	// HAProxy config. will be generated and passed to the proxy. The proxy will
	// restart.
	return strategy.hacfgupdater.UpdateHAConfig(data)
}

// updateProxyConfiguration updates the HAProxy configuration with the provided
// list of deployed functions. HAProxy will always be reloaded after the update.
//
// weights can be nil if phase is allLocalPhase.
func (strategy *RLAgentStrategy) updateProxyConfiguration(funcs map[string]*uint, weights map[string]map[string]uint, phase strategyPhase) error {
	neighbors := make(map[string]map[string]string)
	for _, peer := range _p2pHost.Network().Peers() {
		host, err := extractSingleIPv4(_p2pHost, peer)
		if err != nil {
			return fmt.Errorf("failed to build neighbors information: %w", err)
		}

		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)
		neighbors[peerID] = make(map[string]string)
		neighbors[peerID]["host"] = host
		// FIXME: The remote port may change!
		neighbors[peerID]["port"] = fmt.Sprintf("%d", _config.HAProxyPort)
	}

	// Define and populate this anonymous struct to pass data to the Go
	// template.
	data := struct {
		Now          string
		DFaaSNodeID  string
		Functions    map[string]*uint
		Weights      map[string]map[string]uint
		Neighbors    map[string]map[string]string
		OpenFaaSHost string
		OpenFaaSPort uint
		RejectorHost string
		RejectorPort uint
		Phase        string
	}{
		Now:          time.Now().UTC().Format("2006-01-02 15:04:05 MST"),
		DFaaSNodeID:  _p2pHost.ID().String(),
		Functions:    funcs,
		Weights:      weights,
		Neighbors:    neighbors,
		OpenFaaSHost: _config.OpenFaaSHost,
		OpenFaaSPort: _config.OpenFaaSPort,
		RejectorHost: _config.RejectorHost,
		RejectorPort: _config.RejectorPort,
		Phase:        phase.String(),
	}

	return strategy.hacfgupdater.UpdateHAConfig(data)
}

func (strategy *RLAgentStrategy) buildObservation() ([]byte, error) {
	if strategy.allLocalPhaseTimestamp.IsZero() {
		return nil, errors.New("allLocalPhaseTimestamp is not set, but is required for rlAgentPhase")
	}
	now := time.Now().UTC()
	if now.Before(strategy.allLocalPhaseTimestamp) {
		return nil, errors.New("allLocalPhaseTimestamp cannot be greater than time.Now()!")
	}

	obs := make(map[string]any)

	// input_rate key in observation.
	inputRPS, err := strategy.promq.InputRPS(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'input_rate' key: %w", err)
	}
	// Because we currently support only one function!
	inputRPSSingle, err := extractSingleFunctionValue(inputRPS, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'input_rate' key: %w", err)
	}
	obs["input_rate"] = inputRPSSingle

	// previous_input_rate key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_input_rate"] = 0
	} else {
		inputRPS, err := strategy.promq.InputRPS(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_input_rate' key: %w", err)
		}
		inputRPSSingle, err := extractSingleFunctionValue(inputRPS, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_input_rate' key: %w", err)
		}
		obs["previous_input_rate"] = inputRPSSingle
	}

	// previous_fwd_to_node_X key in observation.
	peers := 0
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		for _, peer := range _p2pHost.Network().Peers() {
			key := fmt.Sprintf("previous_fwd_to_node_%s", peer.String())
			obs[key] = 0
			peers++
		}
	} else {
		prevForwardRPS, err := strategy.promq.ForwardRPS(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: %w", err)
		}
		prevForwardRPSSingle, err := extractSingleFunctionValue(prevForwardRPS, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: %w", err)
		}
		for peer, rate := range prevForwardRPSSingle {
			// ForwardRPS() always returns openfaas-local node that is the
			// local one (not remote), and also rejector.
			if peer == "openfaas-local" || peer == "rejector" {
				continue
			}
			// Peer here is already a string with format "node_X".
			key := fmt.Sprintf("previous_fwd_to_%s", peer)
			obs[key] = rate
			peers++
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: found %d peers, expected 4 peers", peers)
	}

	// previous_fwd_to_node_X_rejected key in observation.
	peers = 0
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		for _, peer := range _p2pHost.Network().Peers() {
			key := fmt.Sprintf("previous_fwd_to_node_%s_rejected", peer.String())
			obs[key] = 0
			peers++
		}
	} else {
		prevForwardRejectRPS, err := strategy.promq.ForwardRejectRPS(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: %w", err)
		}
		prevForwardRejectRPSSingle, err := extractSingleFunctionValue(prevForwardRejectRPS, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: %w", err)
		}
		for peer, rate := range prevForwardRejectRPSSingle {
			// ForwardRejectRPS() always returns openfaas-local node that is the
			// local one (not remote).
			if peer == "openfaas-local" || peer == "rejector" {
				continue
			}
			// Peer here is already a string with format "node_X".
			key := fmt.Sprintf("previous_fwd_to_%s_rejected", peer)
			obs[key] = rate
			peers++
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: found %d peers, expected 4 peers", peers)
	}

	// reject_rate key in observation.
	rejectRate, err := strategy.promq.RejectRate(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	rejectRateSingle, err := extractSingleFunctionValue(rejectRate, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	obs["reject_rate"] = rejectRateSingle

	// previous_reject_rate key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_reject_rate"] = 0.0
	} else {
		prevRejectRate, err := strategy.promq.RejectRate(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_reject_rate' key: %w", err)
		}
		prevRejectRateSingle, err := extractSingleFunctionValue(prevRejectRate, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_reject_rate' key: %w", err)
		}
		obs["previous_reject_rate"] = prevRejectRateSingle
	}

	// avg_resp_time_loc key in observation.
	avgRespTime, err := strategy.promq.AvgRespTimeLocal(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'avg_resp_time_loc' key: %w", err)
	}
	avgRespTimeSingle, err := extractSingleFunctionValue(avgRespTime, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'avg_resp_time_loc' key: %w", err)
	}
	obs["avg_resp_time_loc"] = avgRespTimeSingle

	// previous_avg_resp_time_loc key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_avg_resp_time_loc"] = 0.0
	} else {
		prevAvgRespTime, err := strategy.promq.AvgRespTimeLocal(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_loc' key: %w", err)
		}
		prevAvgRespTimeSingle, err := extractSingleFunctionValue(prevAvgRespTime, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_loc' key: %w", err)
		}
		obs["previous_avg_resp_time_loc"] = prevAvgRespTimeSingle
	}

	// previous_avg_resp_time_fwd_to_node_X key in observation.
	peers = 0
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		for _, peer := range _p2pHost.Network().Peers() {
			key := fmt.Sprintf("previous_avg_resp_time_fwd_to_node_%s", peer.String())
			obs[key] = 0
			peers++
		}
	} else {
		prevAvgRespTimeForward, err := strategy.promq.AvgRespTimeForward(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: %w", err)
		}
		prevAvgRespTimeForwardSingle, err := extractSingleFunctionValue(prevAvgRespTimeForward, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: %w", err)
		}
		for peer, rate := range prevAvgRespTimeForwardSingle {
			// AvgRespTimeForward() always returns openfaas-local node that is
			// the local one (not remote).
			if peer == "openfaas-local" || peer == "rejector" {
				continue
			}
			// peer here is a string with format "node_<id>".
			key := fmt.Sprintf("previous_avg_resp_time_fwd_to_%s", peer)
			obs[key] = rate
			peers++
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: found %d peers, expected 4 peers", peers)
	}

	// cpu_utilization key in observation (float32 in [0, 1]).
	cpuUsage, err := strategy.promq.CPUUsage(strategy.targetFunction, strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'cpu_utilization' key: %w", err)
	}
	obs["cpu_utilization"] = cpuUsage / 100

	// previous_cpu_utilization key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_cpu_utilization"] = 0.0
	} else {
		prevCPUUsage, err := strategy.promq.CPUUsage(strategy.targetFunction, strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_cpu_utilization' key: %w", err)
		}
		obs["previous_cpu_utilization"] = prevCPUUsage / 100
	}

	// n_replicas key observation.
	replicas, err := strategy.promq.Replicas(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	replicasSingle, err := extractSingleFunctionValue(replicas, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	obs["n_replicas"] = replicasSingle

	// previous_n_replicas key observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_n_replicas"] = 1
	} else {
		prevReplicas, err := strategy.promq.Replicas(strategy.rlAgentPhaseTimestamp, strategy.allLocalPhaseTimestamp)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_n_replicas' key: %w", err)
		}
		prevReplicasSingle, err := extractSingleFunctionValue(prevReplicas, strategy.targetFunction)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_n_replicas' key: %w", err)
		}
		obs["previous_n_replicas"] = prevReplicasSingle
	}

	node_id := fmt.Sprintf("node_%s", _p2pHost.ID().String())
	payload := map[string]any{
		"observation": map[string]any{node_id: obs},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build JSON observation: %w", err)
	}
	return data, nil
}

// queryRLModel queries the RL model with the given observation already
// marshalled into a JSON and returns the action result.
//
// The returned map contains the node ID as first-level key, in the form
// "node_ID" (typically only one entry). The second-level keys represent
// actions: "local" is the proportion of requests to enqueue locally, "reject"
// is the proportion of requests to reject, and zero or more "node_X" with the
// proportion of requests to forward to the node with ID X. The sum of all
// proportions is equal to 1.
func (strategy *RLAgentStrategy) queryRLModel(observation []byte) (map[string]map[string]float64, error) {
	url := fmt.Sprintf("http://%s:%d/action", strategy.rlModelHost, strategy.rlModelPort)

	logger := logging.Logger()
	logger.Debugf("Observation JSON to send to RL model: %s", string(observation))

	resp, err := strategy.httpClient.Post(url, "application/json", bytes.NewReader(observation))
	if err != nil {
		return nil, fmt.Errorf("querying RL model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP POST request returned %d with body %q", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response: %w", err)
	}

	logger.Debugf("Action JSON from response: %s", string(body))

	debugRLModelToFile(rlModelLogPath, string(observation), string(body))

	var action map[string]map[string]float64
	err = json.Unmarshal(body, &action)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling HTTP response to JSON: %w", err)
	}

	return action, nil
}

// applyAction applies the given agent action to the proxy by setting the
// weights for each sub-action.
//
// The input map is structured with the function name as the first-level key.
// Under each function, actions are specified as second-level keys, where the
// action can be "local" for local processing, "reject" for direct rejection,
// or "node_XYZ" for forwarding to a node with the corresponding ID ZYX.
//
// Each action key maps to a float value in the range [0, 1], representing the
// proportion (weight) assigned to that action.
//
// Warning: currenlty only one function is supported in the given map,
// controlled by strategy.targetFunction.
func (strategy *RLAgentStrategy) applyAction(action map[string]map[string]float64) error {
	logger := logging.Logger()

	// We need the list of neighbors for later use.
	neighbors := []string{}
	for _, peer := range _p2pHost.Network().Peers() {
		// "node_ID" is the required format.
		peerID := fmt.Sprintf("node_%s", peer)

		neighbors = append(neighbors, peerID)
	}

	// The action map should contain only one key, that's the local node ID. We
	// need to do this check to make sure to have the right action for the right
	// node.
	if len(action) != 1 {
		return fmt.Errorf("found %d node IDs in RL action, expected 1", len(action))
	}

	localNode := fmt.Sprintf("node_%s", _p2pHost.ID())
	if _, exist := action[localNode]; !exist {
		return fmt.Errorf("local node ID not found in RL action. Node ID: %s", localNode)
	}

	backend := fmt.Sprintf("function_%s", strategy.targetFunction)

	// First extract all actions, then apply them.
	weights := make(map[string]uint)

	// Extract local action.
	localProportion, exist := action[localNode]["local"]
	if !exist {
		return fmt.Errorf("local proportion not found in RL action: %v", action)
	}
	weights["openfaas-local"] = uint(math.Round(localProportion * 100))

	// Extract reject action.
	rejectProportion, exist := action[localNode]["reject"]
	if !exist {
		return fmt.Errorf("reject proportion not found in RL action: %v", action)
	}
	weights["rejector"] = uint(math.Round(rejectProportion * 100))

	// Extract forward actions.
	for _, neighborID := range neighbors {
		forwardProportion, exist := action[localNode][neighborID]
		if !exist {
			return fmt.Errorf("neighbor node not found in RL action for forward: %s", neighborID)
		}
		weights[neighborID] = uint(math.Round(forwardProportion * 100))
	}

	// Apply all weights.
	for target, weight := range weights {
		if err := strategy.runtimeapi.SetWeight(backend, target, weight); err != nil {
			return fmt.Errorf("failed to set weight for %s on backend %s: %w", target, backend, err)
		}
	}
	logger.Debugf("HAProxy updated with the following weights: %v", weights)

	return nil
}

// extractSingleFunctionValue extracts data related to the given function name
// from the funcs map.
//
// Required by the RL Agent strategy because it supports only one deployed
// function.
func extractSingleFunctionValue[T any](funcs map[string]T, name string) (T, error) {
	value, exists := funcs[name]
	if !exists {
		var zero T
		return zero, fmt.Errorf("missing function %q in %v", name, funcs)
	}
	return value, nil
}

func extractSingleIPv4(h host.Host, p peer.ID) (string, error) {
	conns := h.Network().ConnsToPeer(p)

	for _, conn := range conns {
		// Get the IPv4 of the active connection, not the cached one in
		// Peerstore!
		remoteAddr := conn.RemoteMultiaddr()

		if ip, err := remoteAddr.ValueForProtocol(ma.P_IP4); err == nil {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found on active connections for peer: %s", p)
}
