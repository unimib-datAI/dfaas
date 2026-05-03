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
