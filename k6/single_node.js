// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This k6 load test targets a single DFaaS node. That node may have zero, one,
// or multiple neighboring DFaaS nodes. However, all requests are generated for
// and sent exclusively to the single target node.
//
// We test the figlet function with a constant load of 100 requests per second,
// assuming its maximum handling capacity is also 100. Therefore, we expect some
// requests to be rejected or forwarded to neighboring nodes, if any are
// available.

import http from 'k6/http';
import { Counter } from 'k6/metrics';

// This metric tracks information related to DFaaS. For each response, it
// records the following:
//
// - status: the HTTP response status (also available in the http_reqs metric),
// - target: the IP address of the DFaaS node to which the request was sent,
// - x_server: the IP address of the DFaaS node that actually processed the
// request,
// - dfaas_node_id: the ID of the DFaaS node that processed the request.
const dfaasRequests = new Counter('dfaas_requests')

export const options = {
  scenarios: {
    fixed_rate: {
      executor: 'constant-arrival-rate',
      rate: 110, // Can be changed (but modify also VUs)!
      timeUnit: '1s',
      duration: '3m', // Can be changed!
      preAllocatedVUs: 3,
      maxVUs: 5,
    },
  },
};

export default function () {
  // Change the IP if necessary!
  const ip = '10.0.2.38'
  const endpoint = `http://${ip}:30080/function/figlet`;
  const payload = 'Hello world!';
  const params = {
    headers: {
      'Content-Type': 'text/plain',
    },
  };

  let res = http.post(endpoint, payload, params);

  // Note that these headers may be not available.
  const xServer = res.headers['X-Server'];
  const dfaasNodeID = res.headers['Dfaas-Node-Id'];

  dfaasRequests.add(1, {
    status: res.status,
    target: ip,
    x_server: xServer || 'undefined',
    dfaas_node_id: dfaasNodeID || 'undefined',
  });
}
