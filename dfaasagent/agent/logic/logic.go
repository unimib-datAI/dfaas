package logic

import (
	"github.com/bcicen/go-haproxy"
	"github.com/libp2p/go-libp2p-core/host"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/cliflags"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/hacfgupd"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/offuncs"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/ofpromq"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
)

// This package handles the main operational logic of the DFaaSAgent application

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _p2pHost host.Host
var _flags *cliflags.ParsedValues

var _nodestbl *nodestbl.Table
var _hacfgupdater hacfgupd.Updater

var _hasockClient haproxy.HAProxyClient
var _ofpromqClient ofpromq.Client
var _offuncsClient offuncs.Client

// Initialize initializes this package (sets some vars, etc...)
func Initialize(p2pHost host.Host) error {
	_p2pHost = p2pHost
	_flags = cliflags.GetValues()

	_nodestbl = &nodestbl.Table{
		// Set validity to 120% of RecalcPeriod
		EntryValidity: _flags.RecalcPeriod + (_flags.RecalcPeriod / 5),
	}
	_nodestbl.InitTable()

	_hacfgupdater = hacfgupd.Updater{
		HAConfigFilePath: _flags.HAConfigFile,
		CmdOnUpdated:     _flags.HAConfigUpdateCommand,
	}
	err := _hacfgupdater.LoadTemplate(_flags.HATemplateFile)
	if err != nil {
		return err
	}

	_hasockClient = haproxy.HAProxyClient{
		Addr: _flags.HASockPath,
	}

	_ofpromqClient = ofpromq.Client{
		Hostname: _flags.PrometheusHost,
		Port:     _flags.PrometheusPort,
	}

	_offuncsClient = offuncs.Client{
		Hostname: _flags.OpenFaaSHost,
		Port:     _flags.OpenFaaSPort,
		Username: _flags.OpenFaaSUser,
		Password: _flags.OpenFaaSPass,
	}

	return nil
}
