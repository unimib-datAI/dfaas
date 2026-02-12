// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.
import http from 'k6/http';

// Requires to track the stage of a request in CSV output.
import { tagWithCurrentStageIndex } from 'https://jslib.k6.io/k6-utils/1.6.0/index.js';

const FUNCTION_URL = 'http://10.0.2.38:30080/function/figlet';
const BODY_CONTENT = 'Ciao';

// Read the trace path from the TRACE_PATH env variable.
if (!__ENV.TRACE_PATH) {
    throw new Error("Missing environment variable TRACE_PATH");
}
const TRACE_PATH = __ENV.TRACE_PATH;

// Read JSON trace file from disk.
const TRACES = JSON.parse(open(TRACE_PATH));

const FUNCTION = __ENV.FUNCTION || "0";
const NODE = __ENV.NODE || "0";
const LIMIT = parseInt(__ENV.LIMIT) || 0;

// We must validate FUNCTION, NODE and LIMIT because JavaScript won't throw a
// clear error otherwise.
if (!Object.prototype.hasOwnProperty.call(TRACES, FUNCTION)) {
    throw new Error(`Function '${FUNCTION}' not found in '${TRACE_PATH}'`);
}
if (!Object.prototype.hasOwnProperty.call(TRACES[FUNCTION], NODE)) {
    throw new Error(`Node '${FUNCTION}' not found in '${TRACE_PATH}'`);
}

// Read the trace.
let nodeTrace = TRACES[FUNCTION][NODE]

// Optionally trim the trace.
if (LIMIT > 0) {
    if (LIMIT < 0) {
        throw new Error(`Limit '${LIMIT}' must be non-negative`)
    }
    nodeTrace = nodeTrace.slice(0, LIMIT)
}

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
      preAllocatedVUs: 3000,
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
    timeout: "8s",
  };
  
  const response = http.post(FUNCTION_URL, BODY_CONTENT, params);
}
