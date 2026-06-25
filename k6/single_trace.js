// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2026 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.
import http from 'k6/http';

// Required to track the stage of a request in CSV output.
import { tagWithCurrentStageIndex, getCurrentStageIndex } from 'https://jslib.k6.io/k6-utils/1.6.0/index.js';

// Required to store only a single copy of an image for mlimage function.
import { readAll, stagesBuild } from "./utils.js";

// Required to create a custom counter metric.
import { Counter } from 'k6/metrics';

// This is a dummy counter, used only to attach custom tags for each response
// (DFaaS-Forwarded-To and DFaaS-Node-ID tags).
const DFaaSCounter = new Counter('DFaaS');

const IP_SERVER = __ENV.IP_SERVER || "10.12.68.9"

// We first set up the function to call, with the URL and body.
let FUNCTION_NAME = __ENV.FUNCTION_NAME || "mlimage";
const FUNCTION_URL = `http://${IP_SERVER}:30080/function/${FUNCTION_NAME}`;

let CONTENT_TYPE, BODY_CONTENT, DATA_PATH;
switch (FUNCTION_NAME) {
  case "figlet":
    CONTENT_TYPE = "text/plain";
    BODY_CONTENT = "Hello World!";
    break;
  case "sentimentanalysis":
    CONTENT_TYPE = "text/plain";
    BODY_CONTENT = "I like Python, but sometimes its syntax feels awkward and its error messages can be unclear.";
    break;
  case "mlimage":
    CONTENT_TYPE = "image/jpeg";
    (async function () {
      BODY_CONTENT = await readAll("mlimage_vulture.jpg");
    })();
    break;
  default:
    throw new Error(`Function ${FUNCTION_NAME} not supported.`)
}

// Read the trace path from the TRACE_PATH env variable.
if (!__ENV.TRACE_PATH) {
  throw new Error("Missing environment variable TRACE_PATH");
}
const TRACE_PATH = __ENV.TRACE_PATH;

// Read JSON trace file from disk.
const TRACES = JSON.parse(open(TRACE_PATH));

const FUNCTION = __ENV.FUNCTION || "0";
const NODE = __ENV.NODE || "node_c";
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

// Build stages according to STAGE_BUILDER env variable. See stagesBuild()
// function inside utils.js for more options.
const STAGE_BUILDER = __ENV.STAGE_BUILDER || "OneMinuteWindow";
const stages = stagesBuild(nodeTrace, STAGE_BUILDER);

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
  // We can ignore the responses' body.
  // See: https://grafana.com/docs/k6/latest/testing-guides/running-large-tests/#save-memory-with-discardresponsebodies
  discardResponseBodies: true,
};

export default function () {
  // Tag each request with its corresponding stage index (e.g., stage 0, 1, 2,
  // ...). This makes it possible to map each request to its stage and, in turn,
  // to the trace iteration.
  //
  // The mapping between stage index and trace iteration depends on the selected
  // STAGE_BUILDER.
  //
  // See: https://grafana.com/docs/k6/latest/using-k6/tags-and-groups/
  tagWithCurrentStageIndex();

  // Add to each requests its current stage index, the rate and duration. This
  // will help the DFaaS node to extract some information about the input trace.
  const stage = getCurrentStageIndex();
  const rate = stages[stage].target;
  const duration = stages[stage].duration;

  const params = {
    headers: {
      "Content-Type": CONTENT_TYPE,
      "DFaaS-K6-Stage": stage,
      "DFaaS-K6-Rate": String(rate),
      "DFaaS-K6-Duration": String(duration),
    },
    timeout: "8s",
  };

  const response = http.post(FUNCTION_URL, BODY_CONTENT, params);

  // Extract the headers added by the DFaaS node and save them as tags on the
  // custom metric!
  //
  // K6 stores header names in canonical form in a case sensitive key-value map.
  // Also by default it stores as array (header can have multiple values).
  //
  // Note we need to check if the header exists and is not empty.
  const nodeId = response.headers["Dfaas-Node-Id"] || "";
  const forwardedTo = response.headers["Dfaas-Forwarded-To"] || "";
  DFaaSCounter.add(1, {DFaaS_Node_ID: nodeId, DFaaS_Forwarded_To: forwardedTo, });
}
