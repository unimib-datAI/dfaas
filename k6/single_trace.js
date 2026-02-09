// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.
import http from 'k6/http';
import { tagWithCurrentStageIndex } from 'https://jslib.k6.io/k6-utils/1.6.0/index.js';

const FUNCTION_URL = 'http://10.0.2.40:30080/function/figlet';
const BODY_CONTENT = 'Ciao';

// Read the trace path from the TRACE_PATH env variable, or use the default one
// if not provided.
const TRACE_PATH = __ENV.TRACE_PATH || './input_requests_traces.json';

// Read JSON trace file from disk.
const traceFile = open(TRACE_PATH)
const traceData = JSON.parse(traceFile);

// Extract first 10 values for function "0" and node "0".
const nodeTrace = traceData["0"]["0"].slice(0, 10);

console.log('Loaded trace for function "0", node "0":', nodeTrace);

// Build stages with 5s transitions and 55s constant rate.
let stages = [];
for (let i = 0; i < nodeTrace.length; i++) {
  stages.push({
    duration: '5s', // 5-second transition to new rate.
    target: Math.round(nodeTrace[i]),
  });
  stages.push({
    duration: '55s', // Keep a constant rate for the remainder of the minute.
    target: Math.round(nodeTrace[i]),
  });
}

export let options = {
  scenarios: {
    trace_node_0: {
      executor: 'ramping-arrival-rate',
      startRate: Math.round(nodeTrace[0]),
      timeUnit: '1s',
      preAllocatedVUs: 1000,
      maxVUs: 50000,
      stages: stages,
    },
  },
};

export default function () {
  // Tag each request with its corresponding stage index (e.g., stage 0, 1, 2,
  // ...). This makes it possible to map each request to its stage and, in turn,
  // to the trace iteration. Note that each trace iteration consists of two
  // stages, so the stage index must be divided by two.
  //
  // See: https://grafana.com/docs/k6/latest/using-k6/tags-and-groups/
  tagWithCurrentStageIndex();

  const params = {
    headers: {
      'Content-Type': 'text/plain',
    },
    timeout: "15s",
  };
  
  const response = http.post(FUNCTION_URL, BODY_CONTENT, params);
}
