package loadbalancer

import (
	"github.com/bcicen/go-haproxy"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/hacfgupd"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/nodestbl"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/forecaster"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/ofpromq"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/infogath/offuncs"
)

// In this file is implemented the Factory creational pattern,
// useful to create the correct Strategy instance

// strategyFactory is the interface which represents a generic strategy factory.
// Every factory for new strategies must implement this interface
type strategyFactory interface {
	// Method to create a new Strategy instance
	createStrategy() (Strategy, error)
}

////////////////// RECALC STRATEGY FACTORY ///////////////////

// Struct representing the factory for Recalc strategy, which implements strategyFactory interface
type recalcStrategyFactory struct {}

// createStrategy creates and returns a new RecalcStrategy instance 
func (strategyFactory *recalcStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &RecalcStrategy{}

	strategy.nodestbl = nodestbl.NewTableRecalc(_config.RecalcPeriod + (_config.RecalcPeriod / 5))

	strategy.hacfgupdater = hacfgupd.Updater{
		HAConfigFilePath: _config.HAProxyConfigFile,
		CmdOnUpdated:     _config.HAProxyConfigUpdateCommand,
	}

	err := strategy.hacfgupdater.LoadTemplate(_config.HAProxyTemplateFileRecalc)
	if err != nil {
		return nil, err
	}

	strategy.hasockClient = haproxy.HAProxyClient{
		Addr: _config.HAProxySockPath,
	}

	strategy.ofpromqClient = ofpromq.Client{
		Hostname: _config.PrometheusHost,
		Port:     _config.PrometheusPort,
	}

	strategy.offuncsClient = offuncs.Client{
		Hostname: _config.OpenFaaSHost,
		Port:     _config.OpenFaaSPort,
		Username: _config.OpenFaaSUser,
		Password: _config.OpenFaaSPass,
	}

	strategy.recalc = recalc{}
	strategy.it = 0

	return strategy, nil
}

////////////////// NODE MARGIN STRATEGY FACTORY ///////////////////

// Struct representing the factory for Node Margin strategy, which implements strategyFactory interface
type nodeMarginStrategyFactory struct {}

// createStrategy creates and returns a new NodeMarginStrategy instance 
func (strategyFactory *nodeMarginStrategyFactory) createStrategy() (Strategy, error) {
	strategy := &NodeMarginStrategy{}

	strategy.nodestbl = nodestbl.NewTableNMS(_config.RecalcPeriod + (_config.RecalcPeriod / 5))

	strategy.hacfgupdater = hacfgupd.Updater{
		HAConfigFilePath: _config.HAProxyConfigFile,
		CmdOnUpdated:     _config.HAProxyConfigUpdateCommand,
	}

	err := strategy.hacfgupdater.LoadTemplate(_config.HAProxyTemplateFileNMS)
	if err != nil {
		return nil, err
	}

	strategy.ofpromqClient = ofpromq.Client{
		Hostname: _config.PrometheusHost,
		Port:     _config.PrometheusPort,
	}

	strategy.offuncsClient = offuncs.Client{
		Hostname: _config.OpenFaaSHost,
		Port:     _config.OpenFaaSPort,
		Username: _config.OpenFaaSUser,
		Password: _config.OpenFaaSPass,
	}

	strategy.forecasterClient = forecaster.Client{
		Hostname: _config.ForecasterHost,
		Port:     _config.ForecasterPort,
	}

	strategy.nodeInfo = nodeInfo{}
	strategy.maxValues = make(map[string]float64)
	strategy.targetNodes = make(map[string][]string)
	strategy.weights = make(map[string]map[string]uint)

	return strategy, nil
}
