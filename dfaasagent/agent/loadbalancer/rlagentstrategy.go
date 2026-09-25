// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
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
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/proxy"
)

type RLAgentStrategy struct {
	hacfgupdater  *hacfgupd.Updater
	runtimeapi    *proxy.RuntimeAPI
	offuncsClient *offuncs.Client
	promq         *promq.Client
	httpClient    *http.Client

	// Connection information for the RL model Web service. Used to get the
	// actions (balacing weights).
	rlModelHost string
	rlModelPort uint

	// Wheter the query for the RL model should ask for exploration of the
	// action space. If set to false, the same observation will return almost
	// the same action. If set to true, the same observation may return
	// different actions.
	rlModelExplore bool

	// The strategy works in conjunction with k6. Time is divided into
	// "iterations", counted from 0 onwards. The strategy will query the RL
	// model at the start of each iteration. These variables are needed to keep
	// track of the previous iteration.
	prevIterationStart time.Time
	prevIterationEnd   time.Time

	// historicalIterationTimeSpan contains all the expected iterations that the
	// strategy will receive. See iterationTimeSpan for more information.
	historicalIterationTimeSpan []iterationTimeSpan

	// historicalPromq is the Prometheus client used to query historical
	// metrics, rather than current metrics as done by promq.
	historicalPromq *promq.Client

	// targetFunction stores the function's name used in the RL Agent strategy
	// (the strategy currently supports only one deployed function).
	targetFunction string

	// The list of neighbors in the "node_ID" format. It will be initialized
	// during the setup, then will be used in the RL Agent phase to iterate over
	// neighbors' metrics.
	neighbors []string
}

// IterationTimeSpan holds the information for a single historical iteration.
// It is used by the strategy to convert the iteration number into the start and
// end time span, and then query the historical Prometheus instance with PromQL
// for the "holistic oracle" during the observation building process. For more
// information, see the buildObservation() function.
type iterationTimeSpan struct {
	Iteration int
	Start     time.Time
	End       time.Time
}

// iterationFromStage returns the current RL Agent iteration based on
// the given k6 stage index.
func iterationFromStage(stage int) (int, error) {
	if stage < 0 {
		return 0, errors.New("found negative stage, expected positive or zero")
	}

	return stage / 2, nil
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
	// We start with a nil iteration, since we do not detected any stage.
	var previousIteration *int

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

		// The first part of the code is dedicated to detecting the stage value,
		// which is then used to detect the iteration.

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
				//logger.Warn("Cannot detect current stage: empty gpt0 field for stick-table main. Skipping running cycle")
				continue
			}
			return fmt.Errorf("reading gpt0 field in stick-table main from HAProxy: %w", err)
		}
		if stage == 0 {
			logger.Warn("Cannot detect current stage: requests do not have DFaaS-K6-Stage header. Skipping running cycle")
			continue
		} else {
			// We must subtract 1 because the HAProxy config always add +1 to
			// the stage counter to be able to represent the value 0 as "no
			// stage header found".
			stage--
		}
		logger.Infof("Current detected stage: %d", stage)

		if previousStage != stage {
			// Initialize the debug file when a new k6 execution starts.
			if stage == 0 {
				if err := debugRLModelToFileInit(rlModelLogPath); err != nil {
					return fmt.Errorf("failed to init log file: %w", err)
				}
			}

			// Also make sure to update previousStage!
			previousStage = stage
		}

		// From the current stage, we determine the iteration.
		//
		// Since k6 is used for load testing, each iteration consists of two
		// internal stages.
		currentIteration, err := iterationFromStage(stage)
		if err != nil {
			logger.Warnf("Failed converting detected stage to k6 iteration, skipping running cycle: %w", err)
			continue
		}

		if previousIteration == nil {
			logger.Infof("Starting strategy with iteration number %d", currentIteration)
		} else if *previousIteration == currentIteration {
			logger.Infof("Current iteration is equal to previous (%d). Nothing to do.", currentIteration)
			continue
		} else {
			logger.Infof("Moving from iteration number %d to %d", *previousIteration, currentIteration)
		}

		err = strategy.runIteration(currentIteration)
		if err != nil {
			return fmt.Errorf("running cycle with iteration number %d: %w", currentIteration, err)
		}

		previousIteration = &currentIteration

		duration := time.Since(start)
		httpserver.StrategyIterationDuration.Set(duration.Seconds())
		httpserver.StrategySuccessIterations.Inc()
		logger.Infof("Running cycle completed. Duration: %s", duration.String())
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
		return fmt.Errorf("failed to check deployed functions: %w", err)
	}
	if len(funcs) == 0 || len(funcs) > 1 {
		logger.Errorf("Strategy requires exactly one deployed function, found %d functions", len(funcs))
		return fmt.Errorf("exactly one function required, found %d", len(funcs))
	}
	strategy.targetFunction = funcs[0]
	logger.Infof("Target function of RL Agent strategy is %q", strategy.targetFunction)

	// We first set the initial proxy configuration. runIteration will update
	// the weights based on this configuration.
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

// runIteration builds the observation for the RL model, queries the model, and
// applies the resulting action to the proxy.
func (strategy *RLAgentStrategy) runIteration(currentIteration int) error {
	obs, err := strategy.buildObservation(currentIteration)
	if err != nil {
		return fmt.Errorf("building observation: %w", err)
	}

	action, err := strategy.queryRLModel(obs)
	if err != nil {
		return fmt.Errorf("querying RL model: %w", err)
	}

	if err := strategy.applyAction(action); err != nil {
		return fmt.Errorf("applying RL action: %w", err)
	}

	strategy.prevIterationStart = strategy.prevIterationEnd

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
	strategy.neighbors = nil                 // Node's ID (node_XXX).
	neighborsPort := make(map[string]string) // Node's ID -> Node's host addr.
	neighborsHost := make(map[string]string) // Node's ID -> Node's port numb.

	for _, peer := range _p2pHost.Network().Peers() {
		host, err := extractSingleIPv4(_p2pHost, peer)
		if err != nil {
			return fmt.Errorf("failed to build neighbors information: %w", err)
		}

		// "node_ID" is required format.
		peerID := fmt.Sprintf("node_%s", peer)
		fmt.Printf("Detected node: %q", peerID)

		strategy.neighbors = append(strategy.neighbors, peerID)
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
		Neighbors:     strategy.neighbors,
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

func (strategy *RLAgentStrategy) buildObservation(currentIteration int) ([]byte, error) {
	// During the first iteration, there is no previous iteration, so the
	// queries to the real-time Prometheus instance must be skipped (they do not
	// exist yet).
	firstIteration := false
	if strategy.prevIterationStart.IsZero() && strategy.prevIterationEnd.IsZero() {
		firstIteration = true
	}

	// Sanity check valid only from the 2nd iteration onwards.
	if !firstIteration {
		if strategy.prevIterationStart.IsZero() {
			return nil, errors.New("prevIterationStart must be set")
		}
	}

	// Even if this is the first iteration, we set this variable, but it will
	// not be used in this call.
	strategy.prevIterationEnd = time.Now().UTC()
	if strategy.prevIterationEnd.Before(strategy.prevIterationStart) {
		return nil, errors.New("prevIterationEnd cannot be before than prevIterationStart")
	}

	obs := make(map[string]any)

	// Observation information is taken from two Prometheus sources: one
	// contains real-time measurements, while the other contains historical
	// measurements used as a "holistic oracle" for the next incoming iteration.
	// The latter requires converting the iteration number into a duration span.
	historicalStart, historicalEnd, err := strategy.historicalTimeSpanFrom(currentIteration)
	if err != nil {
		return nil, fmt.Errorf("failed to convert iteration number %d to time span for historical data: %w", currentIteration, err)
	}

	// input_rate key in observation.
	inputRPS, err := strategy.historicalPromq.InputRPS(historicalStart, historicalEnd)
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
	if firstIteration {
		obs["previous_input_rate"] = 0
	} else {
		inputRPS, err := strategy.promq.InputRPS(strategy.prevIterationStart, strategy.prevIterationEnd)
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
	if firstIteration {
		peers := 0
		for _, peer := range strategy.neighbors {
			key := fmt.Sprintf("previous_fwd_to_%s", peer)
			obs[key] = 0
			peers++
		}
		// FIXME: Remove this code (used for debugging).
		if peers != 4 {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: found %d peers, expected 4 peers", peers)
		}
	} else {
		prevForwardRPS, err := strategy.promq.ForwardRPS(strategy.prevIterationStart, strategy.prevIterationEnd)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: %w", err)
		}
		strategy.applyPromQLNeighborMetric(obs, prevForwardRPS, "previous_fwd_to_%s")
	}

	// previous_fwd_to_node_X_rejected key in observation.
	if firstIteration {
		peers := 0
		for _, peer := range strategy.neighbors {
			key := fmt.Sprintf("previous_fwd_to_%s_rejected", peer)
			obs[key] = 0
			peers++
		}
		// FIXME: Remove this code (used for debugging).
		if peers != 4 {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: found %d peers, expected 4 peers", peers)
		}
	} else {
		prevForwardRejectRPS, err := strategy.promq.ForwardRejectRPS(strategy.prevIterationStart, strategy.prevIterationEnd)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: %w", err)
		}
		strategy.applyPromQLNeighborMetric(obs, prevForwardRejectRPS, "previous_fwd_to_%s_rejected")
	}

	// reject_rate key in observation.
	rejectRate, err := strategy.historicalPromq.RejectRate(historicalStart, historicalEnd)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	rejectRateSingle, err := extractSingleFunctionValue(rejectRate, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	obs["reject_rate"] = rejectRateSingle

	// previous_reject_rate key in observation.
	if firstIteration {
		obs["previous_reject_rate"] = 0.0
	} else {
		prevRejectRate, err := strategy.promq.RejectRate(strategy.prevIterationStart, strategy.prevIterationEnd)
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
	avgRespTime, err := strategy.historicalPromq.AvgRespTimeLocal(historicalStart, historicalEnd)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'avg_resp_time_loc' key: %w", err)
	}
	avgRespTimeSingle, err := extractSingleFunctionValue(avgRespTime, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'avg_resp_time_loc' key: %w", err)
	}
	obs["avg_resp_time_loc"] = avgRespTimeSingle

	// previous_avg_resp_time_loc key in observation.
	if firstIteration {
		obs["previous_avg_resp_time_loc"] = 0.0
	} else {
		prevAvgRespTime, err := strategy.promq.AvgRespTimeLocal(strategy.prevIterationStart, strategy.prevIterationEnd)
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
	if firstIteration {
		peers := 0
		for _, peer := range strategy.neighbors {
			key := fmt.Sprintf("previous_avg_resp_time_fwd_to_%s", peer)
			obs[key] = 0
			peers++
		}
		// FIXME: Remove this code (used for debugging).
		if peers != 4 {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: found %d peers, expected 4 peers", peers)
		}
	} else {
		prevAvgRespTimeForward, err := strategy.promq.AvgRespTimeForward(strategy.prevIterationStart, strategy.prevIterationEnd)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: %w", err)
		}
		strategy.applyPromQLNeighborMetric(obs, prevAvgRespTimeForward, "previous_avg_resp_time_fwd_to_%s")
	}

	// cpu_utilization key in observation (float32 in [0, 1]).
	cpuUsage, err := strategy.historicalPromq.CPUUsage(strategy.targetFunction, historicalStart, historicalEnd)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'cpu_utilization' key: %w", err)
	}
	obs["cpu_utilization"] = cpuUsage / 100

	// previous_cpu_utilization key in observation.
	if firstIteration {
		obs["previous_cpu_utilization"] = 0.0
	} else {
		prevCPUUsage, err := strategy.promq.CPUUsage(strategy.targetFunction, strategy.prevIterationStart, strategy.prevIterationEnd)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_cpu_utilization' key: %w", err)
		}
		obs["previous_cpu_utilization"] = prevCPUUsage / 100
	}

	// n_replicas key observation.
	replicas, err := strategy.historicalPromq.Replicas(historicalStart, historicalEnd)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	replicasSingle, err := extractSingleFunctionValue(replicas, strategy.targetFunction)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	obs["n_replicas"] = replicasSingle

	// previous_n_replicas key observation.
	if firstIteration {
		obs["previous_n_replicas"] = 1
	} else {
		prevReplicas, err := strategy.promq.Replicas(strategy.prevIterationStart, strategy.prevIterationEnd)
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
		"observation":      map[string]any{node_id: obs},
		"agent_parameters": map[string]any{"explore": strategy.rlModelExplore},
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
	for _, neighborID := range strategy.neighbors {
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

// applyPromQLNeighborMetric populates the observation map with per-neighbor
// metrics. It takes the PromQL result map and writes values into `obs` using a
// formatted key.
//
// `obs_fmt_key` defines the key pattern used for each neighbor (e.g.
// "previous_fwd_to_%s_rejected", which will be rendered as
// "previous_fwd_to_node_XXX_rejected").
func (strategy *RLAgentStrategy) applyPromQLNeighborMetric(obs map[string]any, metric map[string]map[string]float32, obs_fmt_key string) {
	// We have two problems: 1) the metric may not exist, and 2) the metric may
	// exist but not the series for a specific neighbor. In both cases we set by
	// default 0.

	logger := logging.Logger()
	dummy_node := fmt.Sprintf(obs_fmt_key, "node_X") // Used only for warnings.

	metricFunc, err := extractSingleFunctionValue(metric, strategy.targetFunction)
	if err != nil {
		logger.Warnf("for key %q, function %q not found in metrics result, setting 0 by default to all neighbors", dummy_node, strategy.targetFunction)
	}

	for _, peer := range strategy.neighbors {
		key := fmt.Sprintf(obs_fmt_key, peer)
		value := float32(0) // Default value.

		if err == nil {
			// With err == nil we are sure extractSingleFunctionValue has
			// extracted something!
			metricValue, exists := metricFunc[peer]
			if exists {
				value = metricValue
			} else {
				logger.Warnf("for key %q and peer %q, function %q not found in metrics result, setting 0 by default to this neighbor", dummy_node, peer, strategy.targetFunction)
			}
		}

		obs[key] = value
	}
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

// readIterationTimestamps reads the iteration timestamp mapping from a CSV file.
//
// The CSV file is expected to contain a header row followed by entries with the
// following columns: iteration, start_timestamp_s, and end_timestamp_s.
//
// Timestamps are Unix timestamps expressed in seconds and are converted to
// time.Time values in UTC.
//
// The function returns a slice containing the time span for each iteration, or
// an error if the file cannot be opened or a CSV entry cannot be parsed.
func readIterationTimestamps(path string) ([]iterationTimeSpan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(bytes.NewReader(data))

	// Skip header.
	if _, err := reader.Read(); err != nil {
		return nil, err
	}

	// Cycle through all rows (iterations).
	var iterations []iterationTimeSpan
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(record) < 3 {
			return nil, fmt.Errorf("invalid CSV record: expected at least 3 columns, got %d", len(record))
		}

		// Iteration number (iteration).
		iteration, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, fmt.Errorf("invalid iteration value %q: %w", record[0], err)
		}

		// Start time as timestamp (start_timestamp_s).
		startTimestamp, err := strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid start timestamp %q: %w", record[1], err)
		}

		// End time as timestamp (end_timestamp_s).
		endTimestamp, err := strconv.ParseInt(record[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid end timestamp %q: %w", record[2], err)
		}

		iterations = append(iterations, iterationTimeSpan{
			Iteration: iteration,
			Start:     time.Unix(startTimestamp, 0).UTC(),
			End:       time.Unix(endTimestamp, 0).UTC(),
		})
	}

	return iterations, nil
}

// historicalTimeSpanFrom returns the historical Prometheus time range
// associated with the given iteration.
func (strategy *RLAgentStrategy) historicalTimeSpanFrom(iteration int) (time.Time, time.Time, error) {
	if iteration < 0 || iteration >= len(strategy.historicalIterationTimeSpan) {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid historical iteration number %d", iteration)
	}

	span := strategy.historicalIterationTimeSpan[iteration]

	// This is a redundant check, but it can be useful for detecting malformed
	// data.
	if span.Iteration != iteration {
		return time.Time{}, time.Time{}, fmt.Errorf("historical iteration mismatch: requested %d, found %d", iteration, span.Iteration)
	}

	return span.Start, span.End, nil
}
