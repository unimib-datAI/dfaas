// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package loadbalancer

import (
	"context"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/communication"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/logging"
)



// periodicRunner runs a PeriodicStrategy on a fixed ticker.
type periodicRunner struct {
	s PeriodicStrategy
}

func newPeriodicRunner(s PeriodicStrategy) StrategyRunner {
	return &periodicRunner{s: s}
}

func (r *periodicRunner) Callback() communication.CBOnReceived {
	return r.s.OnReceived
}

func (r *periodicRunner) Run(ctx context.Context) error {
	logger := logging.Logger()

	period := r.s.Period()
	if period == 0 {
		period = time.Minute
	}

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.s.Tick(ctx); err != nil {
				logger.Warnf("periodic runner: Tick error (continuing): %v", err)
			}
		}
	}
}


