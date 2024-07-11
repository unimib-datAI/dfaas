package nodestbl

import (
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/config"
)

// This package is for handling the information about the other nodes in the
// network, such as the max rate limits and the address and port of the relative
// HAProxy server instance

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _config config.Configuration

// Initialize initializes this package
func Initialize(config config.Configuration) {
	_config = config
}
