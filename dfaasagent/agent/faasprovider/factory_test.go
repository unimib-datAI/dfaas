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

func TestNewFaaSProvider_OpenWhisk(t *testing.T) {
	p, err := faasprovider.NewFaaSProvider("openwhisk", "localhost", 3001, "guest", "")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewFaaSProvider_Unknown(t *testing.T) {
	_, err := faasprovider.NewFaaSProvider("fission", "localhost", 8080, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown AGENT_FAAS_PLATFORM")
}
