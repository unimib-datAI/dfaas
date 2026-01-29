import http from 'k6/http';
import { check } from 'k6';

const FUNCTION_URL = 'http://10.0.2.38:30080/function/figlet';
const BODY_CONTENT = 'Ciao';

// Read JSON trace file from disk.
const traceFile = open('./input_requests_traces.json');
const traceData = JSON.parse(traceFile);

// Extract first 10 values for function "0" and node "0".
const nodeTrace = traceData["0"]["0"].slice(0, 10);

console.log('Loaded trace for function "0", node "0":', nodeTrace);

// Build stages with 5s transitions and 55s constant rate.
let stages = [];
for (let i = 0; i < nodeTrace.length; i++) {
  stages.push({
    duration: '5s',  // 5-second transition to new rate.
    target: Math.round(nodeTrace[i]),
  });
  stages.push({
    duration: '55s',  // Keep the same rate for remainder of minute.
    target: Math.round(nodeTrace[i]),
  });
}

export let options = {
  scenarios: {
    trace_based_load: {
      executor: 'ramping-arrival-rate',
      startRate: Math.round(nodeTrace[0]),
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 200,
      stages: stages,
    },
  },
};

export default function () {
  const params = {
    headers: {
      'Content-Type': 'text/plain',
    },
  };
  
  const response = http.post(FUNCTION_URL, BODY_CONTENT, params);
  
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
