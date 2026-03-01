package openwhisk_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider"
	"github.com/unimib-datAI/dfaas/dfaasagent/agent/faasprovider/openwhisk"
)

// mockOpenWhiskServer returns a test server that responds with a fixed action list
// at /api/v1/namespaces/<namespace>/actions.
func mockOpenWhiskServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/") || !strings.HasSuffix(r.URL.Path, "/actions") {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
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

func TestOpenWhiskClientImplementsInterface(t *testing.T) {
	var _ faasprovider.FaaSProvider = openwhisk.New("localhost:8080", "guest", "")
}

func TestGetFuncsWithMaxRates(t *testing.T) {
	srv := mockOpenWhiskServer(t)
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "guest", "")
	rates, err := client.GetFuncsWithMaxRates()
	require.NoError(t, err)
	assert.Equal(t, uint(100), rates["figlet"])
	assert.Equal(t, uint(50), rates["funca"])
}

func TestGetFuncsNames(t *testing.T) {
	srv := mockOpenWhiskServer(t)
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "guest", "")
	names, err := client.GetFuncsNames()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"figlet", "funca"}, names)
}

func TestGetFuncsWithTimeout(t *testing.T) {
	srv := mockOpenWhiskServer(t)
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "guest", "")
	timeouts, err := client.GetFuncsWithTimeout()
	require.NoError(t, err)
	assert.Nil(t, timeouts["figlet"])
	require.NotNil(t, timeouts["funca"])
	assert.Equal(t, uint(5000), *timeouts["funca"])
}

func TestHealthCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[]")
	}))
	defer srv.Close()

	client := openwhisk.New(srv.Listener.Addr().String(), "guest", "")
	status, err := client.HealthCheck()
	require.NoError(t, err)
	assert.Equal(t, "200 OK", status)
}

// mockPrometheusServer returns a test HTTP server that responds with a fixed JSON body.
func mockPrometheusServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

func TestQueryAFET_OpenWhisk(t *testing.T) {
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

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryAFET(1 * time.Minute)
	require.NoError(t, err)
	assert.InDelta(t, 0.042, result["figlet"], 0.001)
}

func TestQueryInvoc_OpenWhisk(t *testing.T) {
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"action": "figlet", "status": "success"},
					"value": [1700000000, "10.5"]
				},
				{
					"metric": {"action": "figlet", "status": "developer_error"},
					"value": [1700000000, "1.2"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryInvoc(1 * time.Minute)
	require.NoError(t, err)
	assert.InDelta(t, 10.5, result["figlet"]["200"], 0.01)
	assert.InDelta(t, 1.2, result["figlet"]["500"], 0.01)
}

func TestQueryServiceCount_OpenWhisk(t *testing.T) {
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"deployment": "figlet"},
					"value": [1700000000, "3"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryServiceCount()
	require.NoError(t, err)
	assert.Equal(t, 3, result["figlet"])
}

func TestQueryCPUusage_OpenWhisk(t *testing.T) {
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"instance": "node1:9100"},
					"value": [1700000000, "0.35"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryCPUusage(1 * time.Minute)
	require.NoError(t, err)
	assert.InDelta(t, 0.35, result["node1:9100"], 0.01)
}

func TestQueryRAMusage_OpenWhisk(t *testing.T) {
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"instance": "node1:9100"},
					"value": [1700000000, "0.72"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryRAMusage(1 * time.Minute)
	require.NoError(t, err)
	assert.InDelta(t, 0.72, result["node1:9100"], 0.01)
}

func TestQueryCPUusagePerFunction_OpenWhisk(t *testing.T) {
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"container": "figlet"},
					"value": [1700000000, "0.15"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryCPUusagePerFunction(1*time.Minute, []string{"figlet"})
	require.NoError(t, err)
	assert.InDelta(t, 0.15, result["figlet"], 0.01)
}

func TestQueryRAMusagePerFunction_OpenWhisk(t *testing.T) {
	promBody := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"container": "figlet"},
					"value": [1700000000, "0.08"]
				}
			]
		}
	}`
	promSrv := mockPrometheusServer(t, promBody)
	defer promSrv.Close()

	client := openwhisk.NewWithPrometheus("localhost:8080", "guest", "", promSrv.Listener.Addr().String())
	result, err := client.QueryRAMusagePerFunction(1*time.Minute, []string{"figlet"})
	require.NoError(t, err)
	assert.InDelta(t, 0.08, result["figlet"], 0.01)
}
