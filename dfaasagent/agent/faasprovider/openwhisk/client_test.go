package openwhisk_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
