package logic

import (
	"github.com/bcicen/go-haproxy"
	"github.com/libp2p/go-libp2p-core/host"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/config"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/hacfgupd"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/offuncs"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/ofpromq"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/forecaster"
)

// This package handles the main operational logic of the DFaaSAgent application

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _p2pHost host.Host
var _config config.Configuration

var _nodestbl *nodestbl.Table
var _hacfgupdater hacfgupd.Updater

var _hasockClient haproxy.HAProxyClient
var _ofpromqClient ofpromq.Client
var _offuncsClient offuncs.Client
var _forecasterClient forecaster.Client

// Initialize initializes this package (sets some vars, etc...)
func Initialize(p2pHost host.Host, config config.Configuration) error {
	_p2pHost = p2pHost
	_config = config

	_nodestbl = &nodestbl.Table{
		// Set validity to 120% of RecalcPeriod
		EntryValidity: _config.RecalcPeriod + (_config.RecalcPeriod / 5),
	}
	_nodestbl.InitTable()

	_hacfgupdater = hacfgupd.Updater{
		HAConfigFilePath: _config.HAProxyConfigFile,
		CmdOnUpdated:     _config.HAProxyConfigUpdateCommand,
	}
	err := _hacfgupdater.LoadTemplate(_config.HAPRoxyTemplateFile)
	if err != nil {
		return err
	}

	_hasockClient = haproxy.HAProxyClient{
		Addr: _config.HAProxySockPath,
	}

	_ofpromqClient = ofpromq.Client{
		Hostname: _config.PrometheusHost,
		Port:     _config.PrometheusPort,
	}

	_offuncsClient = offuncs.Client{
		Hostname: _config.OpenFaaSHost,
		Port:     _config.OpenFaaSPort,
		Username: _config.OpenFaaSUser,
		Password: _config.OpenFaaSPass,
	}

	_forecasterClient = forecaster.Client{
		Hostname: _config.ForecasterHost,
		Port:     _config.ForecasterPort,
	}

	return nil
}
