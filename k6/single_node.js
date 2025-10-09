import http from 'k6/http';
import { Counter } from 'k6/metrics';

const endpoint = 'http://localhost:30080/function/figlet';

const dfaasRequests = new Counter('dfaas_requests')

export const options = {
  scenarios: {
    fixed_rate: {
      executor: 'constant-arrival-rate',
      rate: 110,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 3,
      maxVUs: 5,
    },
  },
};

export default function () {
  const payload = 'Hello world!';
  const params = {
    headers: {
      'Content-Type': 'text/plain',
    },
  };
  let res = http.post(endpoint, payload, params);

  // These headers may be not available.
  const xServer = res.headers['X-Server'];
  const dfaasNodeID = res.headers['Dfaas-Node-Id'];

  dfaasRequests.add(1, {
    status: res.status,
    x_server: xServer || 'undefined',
    dfaas_node_id: dfaasNodeID || 'undefined',
  });
}
