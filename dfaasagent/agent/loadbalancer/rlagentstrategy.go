// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
)

type RLAgentStrategy struct {
	hacfgupdater  *hacfgupd.Updater
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

// RunStrategy handles the execution of the strategy. It is ran in a goroutine.
func (strategy *RLAgentStrategy) RunStrategy() error {
	logger := logging.Logger()
	logger.Debug("Starting RL Agent strategy...")

	logger.Info("Initial set-up: updating proxy with the discovered functions")
	if err := strategy.setup(); err != nil {
		return fmt.Errorf("failed to do initial set-up: %w", err)
	}
	// We start with a nil phase, since we do not detected any stage.
	var previousPhase *strategyPhase

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		start := time.Now()

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
				logger.Warn("Cannot detect current stage: empty gpt0 field for stick-table main. Skipping iteration")
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
		logger.Infof("Iteration completed. Duration: %s", duration.String())
	}

	return nil
}

// OnReceived is executed every time a message from a peer is received.
func (strategy *RLAgentStrategy) OnReceived(msg *pubsub.Message) error {
	return nil
}

// setup() runs the initial setup of the strategy, mainly configuring HAProxy to
// process all incoming requests locally.
func (strategy *RLAgentStrategy) setup() error {
	if err := strategy.setAllLocal(); err != nil {
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

	strategy.allLocalPhaseTimestamp = time.Now()

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

	return nil
}

func (strategy *RLAgentStrategy) setAllLocal() error {
	// Most of the code is taken from alllocal.go
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

	debugFuncs(funcs)
	if err = strategy.updateProxyConfiguration(funcs, nil, allLocalPhase); err != nil {
		return fmt.Errorf("updating proxy config: %w", err)
	}
	return nil
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
		Phase        string
	}{
		Now:          time.Now().Format("2006-01-02 15:04:05"),
		DFaaSNodeID:  _p2pHost.ID().String(),
		Functions:    funcs,
		Weights:      weights,
		Neighbors:    neighbors,
		OpenFaaSHost: _config.OpenFaaSHost,
		OpenFaaSPort: _config.OpenFaaSPort,
		Phase:        phase.String(),
	}

	return strategy.hacfgupdater.UpdateHAConfig(data)
}

func (strategy *RLAgentStrategy) buildObservation() ([]byte, error) {
	if strategy.allLocalPhaseTimestamp.IsZero() {
		return nil, errors.New("allLocalPhaseTimestamp is not set, but is required for rlAgentPhase")
	}
	now := time.Now()
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
	inputRPSSingle, err := extractSingleFunctionValue(inputRPS)
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
		inputRPSSingle, err := extractSingleFunctionValue(inputRPS)
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
		prevForwardRPSSingle, err := extractSingleFunctionValue(prevForwardRPS)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: %w", err)
		}
		for peer, rate := range prevForwardRPSSingle {
			// ForwardRPS() always returns openfaas-local node that is the
			// local one (not remote).
			if peer == "openfaas-local" {
				continue
			}
			// Peer here is already a string with format "node_X".
			key := fmt.Sprintf("previous_fwd_to_%s", peer)
			obs[key] = rate
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X' key: found %d peers, expected 4: peers", peers)
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
		prevForwardRejectRPSSingle, err := extractSingleFunctionValue(prevForwardRejectRPS)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: %w", err)
		}
		for peer, rate := range prevForwardRejectRPSSingle {
			// ForwardRejectRPS() always returns openfaas-local node that is the
			// local one (not remote).
			if peer == "openfaas-local" {
				continue
			}
			// Peer here is already a string with format "node_X".
			key := fmt.Sprintf("previous_fwd_to_%s_rejected", peer)
			obs[key] = rate
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_fwd_to_node_X_rejected' key: found %d peers, expected 4: peers", peers)
	}

	// reject_rate key in observation.
	rejectRate, err := strategy.promq.RejectRate(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'reject_rate' key: %w", err)
	}
	rejectRateSingle, err := extractSingleFunctionValue(rejectRate)
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
		prevRejectRateSingle, err := extractSingleFunctionValue(prevRejectRate)
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
	avgRespTimeSingle, err := extractSingleFunctionValue(avgRespTime)
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
		prevAvgRespTimeSingle, err := extractSingleFunctionValue(prevAvgRespTime)
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
		prevAvgRespTimeForwardSingle, err := extractSingleFunctionValue(prevAvgRespTimeForward)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: %w", err)
		}
		for peer, rate := range prevAvgRespTimeForwardSingle {
			// AvgRespTimeForward() always returns openfaas-local node that is
			// the local one (not remote).
			if peer == "openfaas-local" {
				continue
			}
			key := fmt.Sprintf("previous_avg_resp_time_fwd_to_node_%s", peer)
			obs[key] = rate
		}
	}
	// FIXME: Remove this code (used for debugging).
	if peers != 4 {
		return nil, fmt.Errorf("building observation for 'previous_avg_resp_time_fwd_to_node_X' key: found %d peers, expected 4: peers", peers)
	}

	// cpu_utilization key in observation.
	// FIXME: Only one function is supported!
	cpuUsage, err := strategy.promq.CPUUsage(strategy.allLocalPhaseTimestamp,
		now,
		[]string{"mlimage"})
	if err != nil {
		return nil, fmt.Errorf("building observation for 'cpu_utilization' key: %w", err)
	}
	cpuUsageSingle, err := extractSingleFunctionValue(cpuUsage)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'cpu_utilization' key: %w", err)
	}
	obs["cpu_utilization"] = cpuUsageSingle

	// previous_cpu_utilization key in observation.
	if strategy.rlAgentPhaseTimestamp.IsZero() {
		obs["previous_cpu_utilization"] = 0.0
	} else {
		// FIXME: Only one function is supported!
		prevCPUUsage, err := strategy.promq.CPUUsage(strategy.rlAgentPhaseTimestamp,
			strategy.allLocalPhaseTimestamp,
			[]string{"mlimage"})
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_cpu_utilization' key: %w", err)
		}
		prevCPUUsageSingle, err := extractSingleFunctionValue(prevCPUUsage)
		if err != nil {
			return nil, fmt.Errorf("building observation for 'previous_cpu_utilization' key: %w", err)
		}
		obs["previous_cpu_utilization"] = prevCPUUsageSingle
	}

	// n_replicas key observation.
	replicas, err := strategy.promq.Replicas(strategy.allLocalPhaseTimestamp, now)
	if err != nil {
		return nil, fmt.Errorf("building observation for 'n_replicas' key: %w", err)
	}
	replicasSingle, err := extractSingleFunctionValue(replicas)
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
		prevReplicasSingle, err := extractSingleFunctionValue(prevReplicas)
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

	resp, err := strategy.httpClient.Post(url, "application/json", bytes.NewReader(observation))
	if err != nil {
		return nil, fmt.Errorf("querying RL model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rl model returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading RL model response: %w", err)
	}

	var action map[string]map[string]float64
	err = json.Unmarshal(body, &action)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling JSON RL model response: %w", err)
	}

	return action, nil
}

func (strategy *RLAgentStrategy) applyAction(action map[string]map[string]float64) error {
	// The initial part is similar to allLocalPhase.
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
	debugFuncs(funcs)

	if len(action) != 1 {
		return fmt.Errorf("found %d node IDs in RL action, expected 1", len(action))
	}

	localNode := fmt.Sprintf("node_%s", _p2pHost.ID())
	if _, exist := action[localNode]; !exist {
		return fmt.Errorf("local node ID not found in RL action. Node ID: %s", localNode)
	}

	localProportion, exist := action[localNode]["local"]
	if !exist {
		return fmt.Errorf("local proportion not found in RL action: %v", action)
	}
	rejectProportion, exist := action[localNode]["reject"]
	if !exist {
		return fmt.Errorf("reject proportion not found in RL action: %v", action)
	}

	weights := make(map[string]map[string]uint)
	// FIXME: Support more than functions!
	weights["mlimage"] = make(map[string]uint)

	weights["mlimage"]["reject"] = uint(rejectProportion * 100)

	// We need to rescale local and forward proportions because in HAProxy
	// config the reject is handled separately and the local+forward is handled
	// as HAProxy weights.
	usable := 1.0 - rejectProportion
	if usable <= 0 {
		// Should not happen.
		return fmt.Errorf("invalid proportions: reject=%f leaves no usable capacity", rejectProportion)
	}

	rescaledLocal := localProportion / usable
	weights["mlimage"]["local"] = uint(rescaledLocal * 100)

	for _, peer := range _p2pHost.Network().Peers() {
		forwardTo := fmt.Sprintf("node_%s", peer)

		forwardProportion, exist := action[localNode][forwardTo]
		if !exist {
			return fmt.Errorf("peer node not found in RL action for forward: %s", forwardTo)
		}

		rescaledForward := forwardProportion / usable
		weights["mlimage"][forwardTo] = uint(rescaledForward * 100)
	}

	if err = strategy.updateProxyConfiguration(funcs, weights, rlAgentPhase); err != nil {
		return fmt.Errorf("updating proxy config: %w", err)
	}
	return nil
}

// FIXME: Remove this! Use because currently we support observations for the RL
// model with only a single function.
func extractSingleFunctionValue[T any](funcs map[string]T) (T, error) {
	// FIXME: Make configurable!
	name := "mlimage"

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
