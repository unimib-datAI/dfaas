// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

// This is a WIP test!
//
// Original test operator.env:
//
// # Node configuration (IP:port or just IP) for health checks.
// Node configuration (IP:port or just IP) for health checks.
// NODES=172.16.238.10;172.16.238.11;172.16.238.12
//
// # Attack configuration parameters. Each attack is separated by a semicolon.
// ATTACKS_NAME=figlet-light-350;figlet-mid-100;figlet-heavy-100;figlet-mid-500
// # Delay in seconds.
// ATTACKS_DELAY=0;300;300;600
// ATTACKS_TARGET=172.16.238.10/function/figlet;172.16.238.11/function/figlet;172.16.238.12/function/figlet;172.16.238.11/function/figlet
// ATTACKS_METHOD=GET;GET;GET;GET
// ATTACKS_BODY=Hello DFaaS World!;Hello DFaaS World!;Hello DFaaS World!;Hello DFaaS World!
// # How many requests do for each second.
// ATTACKS_RATE=350;100;100;500
// # Attack duration in minutes.
// ATTACKS_DURATION=15;10;10;5
// SKIP_PLOTS=true

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
