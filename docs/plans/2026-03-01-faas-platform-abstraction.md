# FaaS Platform Abstraction (OpenFaaS / OpenWhisk) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the DFaaS agent able to use either OpenFaaS or OpenWhisk as its FaaS backend, selected at runtime via an env var, without changing any load-balancing strategy logic.

**Architecture:** Introduce a `FaaSProvider` interface in a new `faasprovider` package. The existing `offuncs` and `ofpromq` code becomes the `openfaas` concrete implementation (thin wrapper, zero behavioral change). A new `openwhisk` implementation talks to the OpenWhisk REST API and its Prometheus metrics (`openwhisk_action_*`). All four strategy structs and `httpserver` use the interface instead of concrete types. The platform is chosen via a new env var `AGENT_FAAS_PLATFORM`.

**Tech Stack:** Go 1.24, libp2p, Prometheus, HAProxy Data Plane API, OpenWhisk REST API (`/api/v1/namespaces/{ns}/actions`), Helm, Kubernetes.

---

## Prometheus metric mapping reference

| Purpose | OpenFaaS metric | OpenWhisk metric | Label diff |
|---|---|---|---|
| Function execution time | `gateway_functions_seconds_sum/count` | `openwhisk_action_duration_seconds_sum/count` | `function_name` → `action` |
| Invocation rate | `gateway_function_invocation_total` | `openwhisk_action_activations_total` | `function_name` → `action`; `code` (HTTP int) → `status` (`success`/`developer_error`) |
| Replica count | `gateway_service_count` | `kube_deployment_status_replicas{namespace="openwhisk"}` | different semantics |
| CPU (node) | `node_cpu_seconds_total` | same | identical |
| RAM (node) | `node_memory_*` | same | identical |
| CPU/RAM per function | `container_cpu/memory_*{container=~"..."}` | same | identical |

Node-exporter and cAdvisor metrics are **platform-agnostic**; only the three gateway metrics change.

---

### Task 1: Define the `FaaSProvider` interface

**Files:**
- Create: `dfaasagent/agent/faasprovider/provider.go`

**Step 1: Write the file**

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.

// Package faasprovider defines the FaaSProvider interface, which abstracts all
// interactions with the underlying FaaS platform (OpenFaaS, OpenWhisk, …).
package faasprovider

import "time"

// FaaSProvider abstracts all platform-specific interactions needed by the
// DFaaS agent strategies and HTTP server.
type FaaSProvider interface {
	// --- FaaS API ---

	// GetFuncsWithMaxRates returns deployed function names mapped to their
	// dfaas.maxrate value (max requests/s). Used by RecalcStrategy.
	GetFuncsWithMaxRates() (map[string]uint, error)

	// GetFuncsNames returns the list of deployed function names.
	// Used by StaticStrategy and NodeMarginStrategy.
	GetFuncsNames() ([]string, error)

	// GetFuncsWithTimeout returns deployed function names mapped to their
	// dfaas.timeout_ms value in ms, or nil if the label is absent.
	// Used by AllLocalStrategy.
	GetFuncsWithTimeout() (map[string]*uint, error)

	// --- Prometheus: platform-specific gateway metrics ---

	// QueryAFET returns the Average Function Execution Time (seconds) per
	// function over the given time span.
	QueryAFET(timeSpan time.Duration) (map[string]float64, error)

	// QueryInvoc returns the invocation rate per function and per status code
	// over the given time span.
	// Outer key: function name. Inner key: status code string ("200", "500", …).
	QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error)

	// QueryServiceCount returns the number of running replicas per function.
	QueryServiceCount() (map[string]int, error)

	// --- Prometheus: platform-agnostic node metrics ---

	// QueryCPUusage returns CPU usage percentage per node-exporter instance.
	QueryCPUusage(timeSpan time.Duration) (map[string]float64, error)

	// QueryRAMusage returns RAM usage percentage per node-exporter instance.
	QueryRAMusage(timeSpan time.Duration) (map[string]float64, error)

	// QueryCPUusagePerFunction returns CPU usage percentage per function container.
	QueryCPUusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error)

	// QueryRAMusagePerFunction returns RAM usage percentage per function container.
	QueryRAMusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error)

	// --- Health ---

	// HealthCheck returns "200 OK" if the FaaS gateway is reachable, or an
	// error otherwise.
	HealthCheck() (string, error)
}
```

**Step 2: Verify it compiles**

```bash
go build -C dfaasagent ./agent/faasprovider/...
```
Expected: no output (success).

**Step 3: Commit**

```bash
cd dfaasagent
git add agent/faasprovider/provider.go
git commit -m "feat: add FaaSProvider interface"
```

---

### Task 2: OpenFaaS implementation (wraps existing code, zero behavior change)

**Files:**
- Create: `dfaasagent/agent/faasprovider/openfaas/client.go`

This is a **thin wrapper** over the already-working `offuncs` and `ofpromq` packages.
No logic is moved; no behavior changes.

**Step 1: Write the file**

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.

// Package openfaas implements faasprovider.FaaSProvider for OpenFaaS.
package openfaas

import (
	"fmt"
	"net/http"
	"time"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/offuncs"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/infogath/ofpromq"
)

// Client implements faasprovider.FaaSProvider for an OpenFaaS gateway.
type Client struct {
	funcsClient *offuncs.Client
	hostname    string
	port        uint
}

// New returns a new OpenFaaS FaaSProvider.
func New(hostname string, port uint) *Client {
	return &Client{
		funcsClient: offuncs.NewClient(hostname, port),
		hostname:    hostname,
		port:        port,
	}
}

func (c *Client) GetFuncsWithMaxRates() (map[string]uint, error) {
	return c.funcsClient.GetFuncsWithMaxRates()
}

func (c *Client) GetFuncsNames() ([]string, error) {
	return c.funcsClient.GetFuncsNames()
}

func (c *Client) GetFuncsWithTimeout() (map[string]*uint, error) {
	return c.funcsClient.GetFuncsWithTimeout()
}

func (c *Client) QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	return ofpromq.QueryAFET(timeSpan)
}

func (c *Client) QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error) {
	return ofpromq.QueryInvoc(timeSpan)
}

func (c *Client) QueryServiceCount() (map[string]int, error) {
	return ofpromq.QueryServiceCount()
}

func (c *Client) QueryCPUusage(timeSpan time.Duration) (map[string]float64, error) {
	return ofpromq.QueryCPUusage(timeSpan)
}

func (c *Client) QueryRAMusage(timeSpan time.Duration) (map[string]float64, error) {
	return ofpromq.QueryRAMusage(timeSpan)
}

func (c *Client) QueryCPUusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error) {
	return ofpromq.QueryCPUusagePerFunction(timeSpan, funcNames)
}

func (c *Client) QueryRAMusagePerFunction(timeSpan time.Duration, funcNames []string) (map[string]float64, error) {
	return ofpromq.QueryRAMusagePerFunction(timeSpan, funcNames)
}

func (c *Client) HealthCheck() (string, error) {
	strURL := fmt.Sprintf("http://%s:%d/healthz", c.hostname, c.port)
	resp, err := http.Get(strURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Status, nil
}
```

**Step 2: Write a compile-check test**

Create `dfaasagent/agent/faasprovider/openfaas/client_test.go`:

```go
package openfaas_test

import (
	"testing"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openfaas"
)

// TestOpenFaaSClientImplementsInterface verifies at compile time that
// *openfaas.Client satisfies faasprovider.FaaSProvider.
func TestOpenFaaSClientImplementsInterface(t *testing.T) {
	var _ faasprovider.FaaSProvider = openfaas.New("localhost", 8080)
}
```

**Step 3: Run the test**

```bash
go test -C dfaasagent ./agent/faasprovider/openfaas/... -v -run TestOpenFaaSClientImplementsInterface
```
Expected: `PASS`

**Step 4: Commit**

```bash
cd dfaasagent
git add agent/faasprovider/openfaas/
git commit -m "feat: add OpenFaaS FaaSProvider implementation (wraps offuncs/ofpromq)"
```

---

### Task 3: Factory + new config fields

**Files:**
- Modify: `dfaasagent/agent/config/config.go`
- Create: `dfaasagent/agent/faasprovider/factory.go`

**Step 1: Add two fields to `Configuration` in `config.go`**

After line 85 (`PowerThresholdNMS float64`), before the closing `}`:

```go
	// Selects the FaaS platform. Accepted values: "openfaas" (default),
	// "openwhisk".
	FaaSPlatform string `mapstructure:"AGENT_FAAS_PLATFORM"`

	// OpenWhisk namespace to query. Only used when FaaSPlatform is "openwhisk".
	// Defaults to "guest".
	OpenWhiskNamespace string `mapstructure:"AGENT_OPENWHISK_NAMESPACE"`

	// OpenWhisk API key (format "uuid:key"). Only used when FaaSPlatform is
	// "openwhisk".
	OpenWhiskAPIKey string `mapstructure:"AGENT_OPENWHISK_API_KEY"`
```

**Step 2: Write the factory**

Create `dfaasagent/agent/faasprovider/factory.go`:

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.

package faasprovider

import (
	"fmt"

	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openfaas"
	// openwhisk will be imported in Task 7
)

const (
	PlatformOpenFaaS   = "openfaas"
	PlatformOpenWhisk  = "openwhisk"
)

// NewFaaSProvider returns the FaaSProvider for the given platform string.
// host and port are the FaaS gateway coordinates.
// namespace and apiKey are only used for OpenWhisk.
func NewFaaSProvider(platform, host string, port uint, namespace, apiKey string) (FaaSProvider, error) {
	switch platform {
	case "", PlatformOpenFaaS:
		return openfaas.New(host, port), nil
	case PlatformOpenWhisk:
		return nil, fmt.Errorf("OpenWhisk provider not yet implemented; set AGENT_FAAS_PLATFORM=openfaas")
	default:
		return nil, fmt.Errorf("unknown AGENT_FAAS_PLATFORM %q; valid values: %q, %q",
			platform, PlatformOpenFaaS, PlatformOpenWhisk)
	}
}
```

**Step 3: Write a factory test**

Create `dfaasagent/agent/faasprovider/factory_test.go`:

```go
package faasprovider_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider"
)

func TestNewFaaSProvider_OpenFaaS(t *testing.T) {
	p, err := faasprovider.NewFaaSProvider("openfaas", "localhost", 8080, "", "")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewFaaSProvider_EmptyDefaultsToOpenFaaS(t *testing.T) {
	p, err := faasprovider.NewFaaSProvider("", "localhost", 8080, "", "")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewFaaSProvider_Unknown(t *testing.T) {
	_, err := faasprovider.NewFaaSProvider("fission", "localhost", 8080, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown AGENT_FAAS_PLATFORM")
}
```

**Step 4: Run tests**

```bash
go test -C dfaasagent ./agent/faasprovider/... -v
```
Expected: all `PASS`

**Step 5: Commit**

```bash
cd dfaasagent
git add agent/config/config.go agent/faasprovider/factory.go agent/faasprovider/factory_test.go
git commit -m "feat: add FaaSProvider factory and AGENT_FAAS_PLATFORM config field"
```

---

### Task 4: Wire `FaaSProvider` into all four strategy structs

Replace the concrete `*offuncs.Client` field with `faasprovider.FaaSProvider` in every strategy and instantiate it via the factory.

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/recalcstrategy.go`
- Modify: `dfaasagent/agent/loadbalancer/nodemarginstrategy.go`
- Modify: `dfaasagent/agent/loadbalancer/staticstrategy.go`
- Modify: `dfaasagent/agent/loadbalancer/alllocal.go`
- Modify: `dfaasagent/agent/loadbalancer/strategyfactory.go`

**Step 1: Update `recalcstrategy.go`**

In the `RecalcStrategy` struct (line 38-53), replace:
```go
	offuncsClient *offuncs.Client
```
with:
```go
	faasProvider  faasprovider.FaaSProvider
```

In `recalcStep1()` line 154, replace:
```go
	strategy.funcs, err = strategy.offuncsClient.GetFuncsWithMaxRates()
```
with:
```go
	strategy.funcs, err = strategy.faasProvider.GetFuncsWithMaxRates()
```

Remove the `offuncs` import line; add the `faasprovider` import.

**Step 2: Update `nodemarginstrategy.go`**

In the `NodeMarginStrategy` struct (line 40-57), replace:
```go
	offuncsClient    *offuncs.Client
```
with:
```go
	faasProvider     faasprovider.FaaSProvider
```

In `RunStrategy()` (lines 103, 112), replace:
```go
	cpuUsage, err = ofpromq.QueryCPUusage(_config.RecalcPeriod)
	...
	ramUsage, err = ofpromq.QueryRAMusage(_config.RecalcPeriod)
```
with:
```go
	cpuUsage, err = strategy.faasProvider.QueryCPUusage(_config.RecalcPeriod)
	...
	ramUsage, err = strategy.faasProvider.QueryRAMusage(_config.RecalcPeriod)
```

In `publishNodeInfo()` (around line 219, the call to `strategy.offuncsClient.GetFuncsNames()`), replace:
```go
	strategy.nodeInfo.funcs, err = strategy.offuncsClient.GetFuncsNames()
```
with:
```go
	strategy.nodeInfo.funcs, err = strategy.faasProvider.GetFuncsNames()
```

Remove `offuncs` and `ofpromq` imports; add `faasprovider` import.

**Step 3: Update `staticstrategy.go`**

In the `StaticStrategy` struct (line 29-41), replace:
```go
	offuncsClient *offuncs.Client
```
with:
```go
	faasProvider  faasprovider.FaaSProvider
```

In `publishNodeInfo()` (line 161), replace:
```go
	strategy.nodeInfo.funcs, err = strategy.offuncsClient.GetFuncsNames()
```
with:
```go
	strategy.nodeInfo.funcs, err = strategy.faasProvider.GetFuncsNames()
```

Remove `offuncs` import; add `faasprovider` import.

**Step 4: Update `alllocal.go`**

In the `AllLocalStrategy` struct (line 22-28), replace:
```go
	offuncsClient *offuncs.Client
```
with:
```go
	faasProvider  faasprovider.FaaSProvider
```

In `RunStrategy()` (line 47), replace:
```go
	funcs, err := strategy.offuncsClient.GetFuncsWithTimeout()
```
with:
```go
	funcs, err := strategy.faasProvider.GetFuncsWithTimeout()
```

Remove `offuncs` import; add `faasprovider` import.

**Step 5: Update `strategyfactory.go`**

Add the `faasprovider` import. Remove the `offuncs` import.

In `recalcStrategyFactory.createStrategy()` (line 59), replace:
```go
	strategy.offuncsClient = offuncs.NewClient(_config.OpenFaaSHost, _config.OpenFaaSPort)
```
with:
```go
	provider, err := faasprovider.NewFaaSProvider(
		_config.FaaSPlatform, _config.OpenFaaSHost, _config.OpenFaaSPort,
		_config.OpenWhiskNamespace, _config.OpenWhiskAPIKey,
	)
	if err != nil {
		return nil, fmt.Errorf("creating FaaS provider: %w", err)
	}
	strategy.faasProvider = provider
```

Apply the same substitution pattern for the other three factory methods (lines 85, 119, 144), each replacing the `offuncs.NewClient(...)` assignment.

**Step 6: Build to verify**

```bash
go build -C dfaasagent ./...
```
Expected: no errors.

**Step 7: Run existing tests**

```bash
go test -C dfaasagent ./... -v
```
Expected: all previously passing tests still pass.

**Step 8: Commit**

```bash
cd dfaasagent
git add agent/loadbalancer/recalcstrategy.go \
        agent/loadbalancer/nodemarginstrategy.go \
        agent/loadbalancer/staticstrategy.go \
        agent/loadbalancer/alllocal.go \
        agent/loadbalancer/strategyfactory.go
git commit -m "refactor: replace offuncs.Client with FaaSProvider interface in all strategies"
```

---

### Task 5: Migrate `healthCheckOpenFaaS` in `httpserver`

**Files:**
- Modify: `dfaasagent/agent/httpserver/httpserver.go`

The existing `healthCheckOpenFaaS()` private function is hardcoded for OpenFaaS's `/healthz` path.
Replace it with a call to the `FaaSProvider.HealthCheck()` method.

**Step 1: Add a `faasProvider` field to the package-level vars in `httpserver.go`**

After line 27 (`var _config config.Configuration`):
```go
var _faasProvider faasprovider.FaaSProvider
```

Add the `faasprovider` import.

**Step 2: Update `Initialize` to accept the provider**

Change the signature of `Initialize` (line 41) from:
```go
func Initialize(config config.Configuration) {
```
to:
```go
func Initialize(config config.Configuration, provider faasprovider.FaaSProvider) {
```

Add at the end of the function body:
```go
	_faasProvider = provider
```

**Step 3: Update the call site in `agent.go`**

In `dfaasagent/agent/agent.go` (line 245, `httpserver.Initialize(config)`), pass the strategy's provider. Because the strategy is already initialized at this point, you need to expose the provider from the `loadbalancer` package.

Add a package-level getter in `dfaasagent/agent/loadbalancer/loadbalancer.go`:
```go
// GetProvider returns the FaaSProvider used by the current strategy.
// Returns nil if not yet initialized.
func GetProvider() faasprovider.FaaSProvider {
	return _faasProvider // add _faasProvider as a package var set in Initialize()
}
```

In `Initialize()` in `loadbalancer.go` (where `_config` is set), also store the provider after calling `GetStrategyInstance()`:
```go
_faasProvider = strategy.getProvider() // Strategy must expose this
```

Alternatively (simpler, avoids exposing internals): create the provider once in `agent.go` and pass it to both `loadbalancer.Initialize` and `httpserver.Initialize`. Refactor `loadbalancer.Initialize` to accept a provider parameter, removing its internal factory call. This is the recommended approach.

**Step 4: Update `healthzHandler` to use the provider**

Replace lines 87-94 in `httpserver.go`:
```go
	// Check FaaS gateway status
	respStatus, err := _faasProvider.HealthCheck()
	if err != nil || respStatus != "200 OK" {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, "- FaaS gateway not ready.\n")
	} else {
		io.WriteString(w, "- FaaS gateway ready.\n")
	}
```

Delete the now-unused `healthCheckOpenFaaS()` function (lines 97-112).

**Step 5: Build and run tests**

```bash
go build -C dfaasagent ./...
go test -C dfaasagent ./... -v
```
Expected: compiles and tests pass.

**Step 6: Commit**

```bash
cd dfaasagent
git add agent/httpserver/httpserver.go agent/agent.go agent/loadbalancer/loadbalancer.go
git commit -m "refactor: use FaaSProvider.HealthCheck() in httpserver instead of hardcoded OpenFaaS endpoint"
```

---

### Task 6: OpenWhisk API client — function metadata

**Files:**
- Create: `dfaasagent/agent/faasprovider/openwhisk/client.go`
- Create: `dfaasagent/agent/faasprovider/openwhisk/client_test.go`

**Step 1: Write the failing test first**

```go
package openwhisk_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openwhisk"
)

// mockOpenWhiskServer returns a test server that responds with a fixed action list.
func mockOpenWhiskServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actions := []map[string]interface{}{
			{
				"name":      "figlet",
				"namespace": "guest",
				"annotations": []map[string]string{
					{"key": "dfaas.maxrate", "value": "100"},
				},
			},
			{
				"name":      "funca",
				"namespace": "guest",
				"annotations": []map[string]string{
					{"key": "dfaas.maxrate", "value": "50"},
					{"key": "dfaas.timeout_ms", "value": "5000"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(actions)
	}))
}

func TestGetFuncsWithMaxRates(t *testing.T) {
	srv := mockOpenWhiskServer(t)
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "", "")

	rates, err := client.GetFuncsWithMaxRates()
	require.NoError(t, err)
	assert.Equal(t, uint(100), rates["figlet"])
	assert.Equal(t, uint(50), rates["funca"])
}

func TestGetFuncsNames(t *testing.T) {
	srv := mockOpenWhiskServer(t)
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "", "")

	names, err := client.GetFuncsNames()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"figlet", "funca"}, names)
}

func TestGetFuncsWithTimeout(t *testing.T) {
	srv := mockOpenWhiskServer(t)
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "", "")

	timeouts, err := client.GetFuncsWithTimeout()
	require.NoError(t, err)
	assert.Nil(t, timeouts["figlet"])
	require.NotNil(t, timeouts["funca"])
	assert.Equal(t, uint(5000), *timeouts["funca"])
}
```

**Step 2: Run failing test**

```bash
go test -C dfaasagent ./agent/faasprovider/openwhisk/... -v -run TestGetFuncs
```
Expected: `FAIL` — package does not exist yet.

**Step 3: Write minimal implementation**

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2026 The DFaaS Authors. All rights reserved.

// Package openwhisk implements faasprovider.FaaSProvider for Apache OpenWhisk.
package openwhisk

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// owAction is the relevant subset of an OpenWhisk action response.
type owAction struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations []owAnnotation    `json:"annotations"`
}

type owAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (a owAction) annotation(key string) (string, bool) {
	for _, ann := range a.Annotations {
		if ann.Key == key {
			return ann.Value, true
		}
	}
	return "", false
}

// Client implements faasprovider.FaaSProvider for OpenWhisk.
type Client struct {
	// host is "hostname:port" of the OpenWhisk API gateway.
	host      string
	namespace string
	apiKey    string
}

// New returns an OpenWhisk FaaSProvider.
// host must be in "hostname:port" form.
// namespace defaults to "guest" if empty.
// apiKey is the OpenWhisk API key ("uuid:key"); may be empty for open deployments.
func New(host, namespace, apiKey string) *Client {
	if namespace == "" {
		namespace = "guest"
	}
	return &Client{host: host, namespace: namespace, apiKey: apiKey}
}

func (c *Client) doActionsRequest() ([]owAction, error) {
	url := fmt.Sprintf("http://%s/api/v1/namespaces/%s/actions", c.host, c.namespace)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("building OpenWhisk actions request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Basic "+basicAuth(c.apiKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET OpenWhisk actions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET OpenWhisk actions: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OpenWhisk actions response: %w", err)
	}

	var actions []owAction
	if err := json.Unmarshal(body, &actions); err != nil {
		return nil, fmt.Errorf("parsing OpenWhisk actions response: %w", err)
	}
	return actions, nil
}

// basicAuth encodes "uuid:key" to base64.
func basicAuth(apiKey string) string {
	import64 "encoding/base64"
	return base64.StdEncoding.EncodeToString([]byte(apiKey))
}

func (c *Client) GetFuncsWithMaxRates() (map[string]uint, error) {
	actions, err := c.doActionsRequest()
	if err != nil {
		return nil, err
	}
	result := map[string]uint{}
	for _, a := range actions {
		val, ok := a.annotation("dfaas.maxrate")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing dfaas.maxrate for action %q: %w", a.Name, err)
		}
		result[a.Name] = uint(n)
	}
	return result, nil
}

func (c *Client) GetFuncsNames() ([]string, error) {
	actions, err := c.doActionsRequest()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, a := range actions {
		names = append(names, a.Name)
	}
	return names, nil
}

func (c *Client) GetFuncsWithTimeout() (map[string]*uint, error) {
	actions, err := c.doActionsRequest()
	if err != nil {
		return nil, err
	}
	result := map[string]*uint{}
	for _, a := range actions {
		val, ok := a.annotation("dfaas.timeout_ms")
		if !ok {
			result[a.Name] = nil
			continue
		}
		n, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing dfaas.timeout_ms for action %q: %w", a.Name, err)
		}
		v := uint(n)
		result[a.Name] = &v
	}
	return result, nil
}

func (c *Client) HealthCheck() (string, error) {
	url := fmt.Sprintf("http://%s/api/v1/namespaces", c.host)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Status, nil
}

// Stub implementations for Prometheus methods (implemented in Task 7).
func (c *Client) QueryAFET(_ time.Duration) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryAFET not yet implemented for OpenWhisk")
}
func (c *Client) QueryInvoc(_ time.Duration) (map[string]map[string]float64, error) {
	return nil, fmt.Errorf("QueryInvoc not yet implemented for OpenWhisk")
}
func (c *Client) QueryServiceCount() (map[string]int, error) {
	return nil, fmt.Errorf("QueryServiceCount not yet implemented for OpenWhisk")
}
func (c *Client) QueryCPUusage(_ time.Duration) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryCPUusage not yet implemented for OpenWhisk")
}
func (c *Client) QueryRAMusage(_ time.Duration) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryRAMusage not yet implemented for OpenWhisk")
}
func (c *Client) QueryCPUusagePerFunction(_ time.Duration, _ []string) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryCPUusagePerFunction not yet implemented for OpenWhisk")
}
func (c *Client) QueryRAMusagePerFunction(_ time.Duration, _ []string) (map[string]float64, error) {
	return nil, fmt.Errorf("QueryRAMusagePerFunction not yet implemented for OpenWhisk")
}
```

> Note: `basicAuth` uses an inline import alias for `encoding/base64`. Move it to the top-level imports in the actual file.

**Step 4: Run tests to verify they pass**

```bash
go test -C dfaasagent ./agent/faasprovider/openwhisk/... -v -run TestGetFuncs
```
Expected: `PASS`

**Step 5: Compile check**

```bash
go build -C dfaasagent ./agent/faasprovider/openwhisk/...
```

**Step 6: Update factory to wire OpenWhisk**

In `dfaasagent/agent/faasprovider/factory.go`, replace the OpenWhisk case:
```go
	case PlatformOpenWhisk:
		return openwhisk.New(
			fmt.Sprintf("%s:%d", host, port),
			namespace,
			apiKey,
		), nil
```
Add `openwhisk` to the imports.

**Step 7: Commit**

```bash
cd dfaasagent
git add agent/faasprovider/openwhisk/ agent/faasprovider/factory.go
git commit -m "feat: add OpenWhisk FaaSProvider — function metadata (API client)"
```

---

### Task 7: OpenWhisk Prometheus queries — gateway metrics

**Files:**
- Modify: `dfaasagent/agent/faasprovider/openwhisk/client.go`
- Modify: `dfaasagent/agent/faasprovider/openwhisk/client_test.go`

OpenWhisk-specific metrics use different names and label keys than OpenFaaS.
The low-level `ofpromq.Query(query)` function is reused; only the PromQL strings and response-parsing structs differ.

**Step 1: Write failing tests for Prometheus methods**

Add to `client_test.go`:

```go
func mockPrometheusServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

func TestQueryAFET_OpenWhisk(t *testing.T) {
	// Real Prometheus response shape, with openwhisk_action_* metric names
	// and "action" label instead of "function_name".
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"action": "figlet"},
					"value": [1700000000, "0.042"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	// The OpenWhisk client must use this Prometheus server.
	// We inject the address via an option or env var override.
	// For the test, set constants.PrometheusOrigin to the test server address.
	// (See implementation note below.)

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryAFET(1 * time.Minute)
	require.NoError(t, err)
	assert.InDelta(t, 0.042, result["figlet"], 0.001)
}
```

> Implementation note: `ofpromq.Query()` reads `constants.PrometheusOrigin` as a package-level constant. To allow injection in tests, either (a) pass the Prometheus host as a parameter to the OpenWhisk client, or (b) create a small internal `promquery(host, query string)` helper that the OpenWhisk client calls directly. Option (b) avoids touching the existing `ofpromq` package. Use option (b).

**Step 2: Run failing test**

```bash
go test -C dfaasagent ./agent/faasprovider/openwhisk/... -v -run TestQueryAFET
```
Expected: `FAIL` — method returns "not yet implemented".

**Step 3: Define OpenWhisk-specific response structs**

Add to a new file `dfaasagent/agent/faasprovider/openwhisk/promtypes.go`:

```go
package openwhisk

// owAFETResponse is the Prometheus JSON response for openwhisk_action_duration_seconds_*.
type owAFETResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Action string `json:"action"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// owInvocResponse is the Prometheus JSON response for openwhisk_action_activations_total.
// OpenWhisk exposes a "status" label ("success", "developer_error", "internal_error")
// instead of an HTTP code.
type owInvocResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Action string `json:"action"`
				Status string `json:"status"` // "success" | "developer_error" | "internal_error"
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// owServiceCountResponse uses kube_deployment_status_replicas.
type owServiceCountResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric struct {
				Deployment string `json:"deployment"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
```

**Step 4: Implement Prometheus query methods**

Replace the stub implementations in `client.go`:

```go
// promQuery runs a PromQL query against the configured Prometheus host.
func (c *Client) promQuery(query string) (string, error) {
	// Reuse the generic query logic from ofpromq but with our own host.
	url := fmt.Sprintf("http://%s/api/v1/query", c.prometheusHost)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func (c *Client) QueryAFET(timeSpan time.Duration) (map[string]float64, error) {
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf(
		`rate(openwhisk_action_duration_seconds_sum[%s]) / rate(openwhisk_action_duration_seconds_count[%s])`,
		t, t,
	)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var resp owAFETResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("parsing Prometheus AFET response: %w", err)
	}
	result := map[string]float64{}
	for _, item := range resp.Data.Result {
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[item.Metric.Action] = val
	}
	return result, nil
}

func (c *Client) QueryInvoc(timeSpan time.Duration) (map[string]map[string]float64, error) {
	t := fmt.Sprintf("%.0fm", timeSpan.Minutes())
	query := fmt.Sprintf(`rate(openwhisk_action_activations_total[%s])`, t)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var resp owInvocResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("parsing Prometheus invoc response: %w", err)
	}
	result := map[string]map[string]float64{}
	for _, item := range resp.Data.Result {
		if _, ok := result[item.Metric.Action]; !ok {
			result[item.Metric.Action] = map[string]float64{}
		}
		// Map OpenWhisk status to HTTP-like code for strategy compatibility.
		code := owStatusToCode(item.Metric.Status)
		val, _ := strconv.ParseFloat(item.Value[1].(string), 64)
		result[item.Metric.Action][code] = val
	}
	return result, nil
}

// owStatusToCode maps OpenWhisk activation status to an HTTP-like code string.
func owStatusToCode(status string) string {
	if status == "success" {
		return "200"
	}
	return "500"
}

func (c *Client) QueryServiceCount() (map[string]int, error) {
	// Use kube_deployment_status_replicas filtered by the OpenWhisk namespace.
	query := fmt.Sprintf(`kube_deployment_status_replicas{namespace="%s"}`, c.namespace)
	body, err := c.promQuery(query)
	if err != nil {
		return nil, err
	}
	var resp owServiceCountResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("parsing Prometheus service count response: %w", err)
	}
	result := map[string]int{}
	for _, item := range resp.Data.Result {
		val, _ := strconv.Atoi(item.Value[1].(string))
		result[item.Metric.Deployment] = val
	}
	return result, nil
}
```

The node-level metrics (`QueryCPUusage`, `QueryRAMusage`, `QueryCPUusagePerFunction`, `QueryRAMusagePerFunction`) use exactly the same PromQL as OpenFaaS (they query node-exporter and cAdvisor, not the FaaS gateway). Copy them from `ofpromq.go` with `c.promQuery(query)` instead of `ofpromq.Query(query)`.

**Step 5: Add `prometheusHost` field and `NewWithPrometheus` constructor**

```go
type Client struct {
	host           string
	namespace      string
	apiKey         string
	prometheusHost string // e.g. "prometheus-server:80"
}

func New(host, namespace, apiKey string) *Client {
	return NewWithPrometheus(host, namespace, apiKey, constants.PrometheusOrigin)
}

func NewWithPrometheus(host, namespace, apiKey, prometheusHost string) *Client {
	if namespace == "" {
		namespace = "guest"
	}
	return &Client{host: host, namespace: namespace, apiKey: apiKey, prometheusHost: prometheusHost}
}
```

**Step 6: Run tests**

```bash
go test -C dfaasagent ./agent/faasprovider/openwhisk/... -v
```
Expected: all `PASS`.

**Step 7: Commit**

```bash
cd dfaasagent
git add agent/faasprovider/openwhisk/
git commit -m "feat: implement OpenWhisk Prometheus queries (gateway + node metrics)"
```

---

### Task 8: Rename `OpenFaaSHost/Port` → `FaaSHost/FaaSPort` in HAProxy types and templates

This is a mechanical rename. The field names only appear in Go struct definitions and Go template `{{.Field}}` accesses — they are never seen by users or operators.

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/hacfgtypes.go`
- Modify: `dfaasagent/agent/loadbalancer/haproxycfgalllocal.tmpl`
- Modify: `dfaasagent/agent/loadbalancer/haproxycfgrecalc.tmpl`
- Modify: `dfaasagent/agent/loadbalancer/haproxycfgnms.tmpl`
- Modify: `dfaasagent/agent/loadbalancer/haproxycfgstatic.tmpl`
- Modify: `dfaasagent/agent/loadbalancer/recalcstrategy.go` (passes `_config.OpenFaaSHost`/`Port` to `createHACfgObject`)
- Modify: `dfaasagent/agent/loadbalancer/alllocal.go` (anonymous struct has `OpenFaaSHost`/`Port`)

**Step 1: Rename in `hacfgtypes.go`**

In the `HACfg` base struct (lines 18-27), rename:
```go
	OpenFaaSHost string  →  FaaSHost string
	OpenFaaSPort uint    →  FaaSPort uint
```

**Step 2: Update all template files**

In each `.tmpl` file, replace every occurrence of `{{.OpenFaaSHost}}` with `{{.FaaSHost}}` and `{{.OpenFaaSPort}}` with `{{.FaaSPort}}`.

Occurrences per template:
- `haproxycfgalllocal.tmpl`: line 71 (`server openfaas-local {{$.OpenFaaSHost}}:{{$.OpenFaaSPort}}`)
- `haproxycfgrecalc.tmpl`: lines 191 and 237 (`be_system_funcs` and `be_myself` backends)
- `haproxycfgnms.tmpl`: search for `OpenFaaSHost`/`OpenFaaSPort` occurrences
- `haproxycfgstatic.tmpl`: search for `OpenFaaSHost`/`OpenFaaSPort` occurrences

**Step 3: Update Go code that sets these fields**

In `recalcstrategy.go`, `createHACfgObject()` (line 544):
```go
		OpenFaaSHost: openFaaSHost,   →   FaaSHost: openFaaSHost,
		OpenFaaSPort: openFaaSPort,   →   FaaSPort: openFaaSPort,
```

In `alllocal.go`, `updateProxyConfiguration()` anonymous struct (lines 90-102):
```go
		OpenFaaSHost: _config.OpenFaaSHost,  →  FaaSHost: _config.OpenFaaSHost,
		OpenFaaSPort: _config.OpenFaaSPort,  →  FaaSPort: _config.OpenFaaSPort,
```

**Step 4: Build to verify**

```bash
go build -C dfaasagent ./...
```
Expected: no errors.

**Step 5: Commit**

```bash
cd dfaasagent
git add agent/loadbalancer/
git commit -m "refactor: rename OpenFaaSHost/Port to FaaSHost/FaaSPort in HACfg and templates"
```

---

### Task 9: HAProxy path rewrite for OpenWhisk

OpenFaaS functions are invoked at `/function/<name>`.
OpenWhisk actions are invoked at `/api/v1/namespaces/<ns>/actions/<name>`.

HAProxy can transparently rewrite the path so external clients always use `/function/<name>` regardless of the backend platform.

**Files:**
- Modify: `dfaasagent/agent/loadbalancer/hacfgtypes.go`
- Modify: all four `.tmpl` files (add rewrite rule)

**Step 1: Add `FaaSBackendPath` to `HACfg`**

```go
type HACfg struct {
	Now          time.Time
	MyNodeID     string
	NodeIP       string
	HAProxyHost  string
	FaaSHost     string
	FaaSPort     uint
	// FaaSBackendPath is the path prefix to prepend when forwarding to the
	// local FaaS backend. For OpenFaaS: "/function". For OpenWhisk:
	// "/api/v1/namespaces/<namespace>/actions".
	FaaSBackendPath string
}
```

**Step 2: Populate `FaaSBackendPath` in strategy factory**

In the `faasprovider` package, expose a helper:

```go
// BackendPathPrefix returns the URL path prefix used to invoke a function
// for the given platform and namespace.
func BackendPathPrefix(platform, namespace string) string {
	if platform == PlatformOpenWhisk {
		if namespace == "" {
			namespace = "guest"
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/actions", namespace)
	}
	return "/function"
}
```

In the `createHACfgObject` calls in each strategy, pass:
```go
FaaSBackendPath: faasprovider.BackendPathPrefix(_config.FaaSPlatform, _config.OpenWhiskNamespace),
```

**Step 3: Update the `be_myself` backend in each template**

In each template, the backend that routes to the local FaaS instance currently uses the function name in the path implicitly (the frontend routes `/function/<name>` and the backend passes it through). Add an `http-request replace-path` rule in `be_myself` and in each `be_others_func_*`:

```
backend be_myself
    ...
    # Rewrite /function/<name> to platform-specific backend path.
    http-request replace-path ^/function/(.+)$ {{$.FaaSBackendPath}}/\1
    server {{$.NodeIP}} {{$.FaaSHost}}:{{$.FaaSPort}} check
```

This single rule handles all functions and works for both platforms (for OpenFaaS, `FaaSBackendPath = /function`, so the path is unchanged).

**Step 4: Build and validate template rendering**

```bash
go build -C dfaasagent ./...
```

Manually verify HAProxy config output using the lint command from `docs/commands.md`:
```bash
podman run --rm -v ./test.cfg:/etc/haproxy.cfg docker.io/library/haproxy:latest haproxy -c -f /etc/haproxy.cfg
```

**Step 5: Commit**

```bash
cd dfaasagent
git add agent/loadbalancer/hacfgtypes.go \
        agent/loadbalancer/haproxycfg*.tmpl \
        agent/faasprovider/factory.go
git commit -m "feat: add FaaSBackendPath to HACfg for platform-transparent HAProxy path rewrite"
```

---

### Task 10: Helm chart and Kubernetes values

**Files:**
- Modify: `k8s/charts/agent/values.yaml` (check current content and add `AGENT_FAAS_PLATFORM`)
- Create: `k8s/charts/values-openwhisk.yaml`
- Create: `k8s/charts/values-prometheus-openwhisk.yaml`

**Step 1: Read existing agent values**

```bash
cat k8s/charts/agent/values.yaml
```

Add the new env vars in the `config:` section:
```yaml
config:
  AGENT_FAAS_PLATFORM: "openfaas"       # "openfaas" | "openwhisk"
  AGENT_OPENWHISK_NAMESPACE: "guest"    # Only used when AGENT_FAAS_PLATFORM=openwhisk
  AGENT_OPENWHISK_API_KEY: ""           # Only used when AGENT_FAAS_PLATFORM=openwhisk
```

**Step 2: Create `values-openwhisk.yaml`**

```yaml
# Helm values to deploy OpenWhisk instead of OpenFaaS.
# Usage:
#   helm install owdev owdev/openwhisk --values k8s/charts/values-openwhisk.yaml
#
# Reference: https://github.com/apache/openwhisk-deploy-kube

whisk:
  ingress:
    apiHostName: "localhost"
    apiHostPort: 31001
    apiHostProto: "http"
    type: NodePort
    nodePort: 31001

# Prometheus scrape job for the OpenWhisk controller (add to your Prometheus config):
# - job_name: 'openwhisk'
#   static_configs:
#     - targets: ['owdev-controller.openwhisk.svc.cluster.local:9090']
```

**Step 3: Create `values-prometheus-openwhisk.yaml`**

This overrides the OpenFaaS-specific alert rule in `values-prometheus.yaml`:
```yaml
# Prometheus values override for OpenWhisk platform.
# Apply with: helm upgrade prometheus ... --values values-prometheus.yaml --values values-prometheus-openwhisk.yaml

serverFiles:
  alerting_rules.yml:
    groups:
      - name: dfaas-openwhisk
        rules:
          - alert: FunctionHighInvocationRate
            expr: rate(openwhisk_action_activations_total[1m]) > 100
            for: 1m
            labels:
              severity: warning
            annotations:
              summary: "High invocation rate on OpenWhisk action {{ $labels.action }}"
```

**Step 4: Update `docs/commands.md` with OpenWhisk deploy snippet**

Add a new section to `docs/commands.md`:
```markdown
## Deploy with OpenWhisk instead of OpenFaaS

```console
$ helm install owdev owdev/openwhisk --values k8s/charts/values-openwhisk.yaml
$ helm install agent ./k8s/charts/agent/ --values values.yaml \
    --set config.AGENT_FAAS_PLATFORM=openwhisk \
    --set config.AGENT_OPENFAAS_HOST=owdev-nginx.openwhisk \
    --set config.AGENT_OPENFAAS_PORT=80
```
```

**Step 5: Commit**

```bash
git add k8s/charts/agent/values.yaml \
        k8s/charts/values-openwhisk.yaml \
        k8s/charts/values-prometheus-openwhisk.yaml \
        docs/commands.md
git commit -m "feat: add Helm values and Prometheus config for OpenWhisk platform"
```

---

### Task 11: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

Add the new env vars and the OpenWhisk deployment commands to the relevant sections.

**Step 1: Edit `CLAUDE.md`**

Under "Offloading Strategies", add a note:
> The FaaS platform is selected via `AGENT_FAAS_PLATFORM` (`openfaas` default, `openwhisk`). When using OpenWhisk, also set `AGENT_OPENWHISK_NAMESPACE` and `AGENT_OPENWHISK_API_KEY`.

Under "Building and Running → Deploying to Kubernetes", add the OpenWhisk variant from the commands added in Task 10.

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with OpenWhisk platform configuration"
```

---

## Summary of new/changed files

| File | Action |
|---|---|
| `dfaasagent/agent/faasprovider/provider.go` | **New** — `FaaSProvider` interface |
| `dfaasagent/agent/faasprovider/factory.go` | **New** — platform factory |
| `dfaasagent/agent/faasprovider/factory_test.go` | **New** |
| `dfaasagent/agent/faasprovider/openfaas/client.go` | **New** — wraps `offuncs` + `ofpromq` |
| `dfaasagent/agent/faasprovider/openfaas/client_test.go` | **New** |
| `dfaasagent/agent/faasprovider/openwhisk/client.go` | **New** — OpenWhisk implementation |
| `dfaasagent/agent/faasprovider/openwhisk/promtypes.go` | **New** — response structs |
| `dfaasagent/agent/faasprovider/openwhisk/client_test.go` | **New** |
| `dfaasagent/agent/config/config.go` | **Modify** — add 3 fields |
| `dfaasagent/agent/loadbalancer/hacfgtypes.go` | **Modify** — rename fields, add `FaaSBackendPath` |
| `dfaasagent/agent/loadbalancer/strategyfactory.go` | **Modify** — use factory |
| `dfaasagent/agent/loadbalancer/recalcstrategy.go` | **Modify** — use interface |
| `dfaasagent/agent/loadbalancer/nodemarginstrategy.go` | **Modify** — use interface + remove direct `ofpromq` calls |
| `dfaasagent/agent/loadbalancer/staticstrategy.go` | **Modify** — use interface |
| `dfaasagent/agent/loadbalancer/alllocal.go` | **Modify** — use interface |
| `dfaasagent/agent/loadbalancer/haproxycfg*.tmpl` (×4) | **Modify** — rename vars + path rewrite rule |
| `dfaasagent/agent/httpserver/httpserver.go` | **Modify** — use `FaaSProvider.HealthCheck()` |
| `dfaasagent/agent/agent.go` | **Modify** — create provider once, pass to both packages |
| `k8s/charts/agent/values.yaml` | **Modify** — add new env vars |
| `k8s/charts/values-openwhisk.yaml` | **New** |
| `k8s/charts/values-prometheus-openwhisk.yaml` | **New** |
| `docs/commands.md` | **Modify** — OpenWhisk deploy commands |
| `CLAUDE.md` | **Modify** |

Existing `offuncs` and `ofpromq` packages are **not deleted** — the OpenFaaS provider wraps them. They can be removed in a follow-up cleanup once the abstraction is stable.
