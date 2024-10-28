package loadbalancer

import (
	"sync"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-core/host"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/config"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/constants"
)

// This package handles the main operational logic of the DFaaSAgent application

//////////////////// MAIN PRIVATE VARS AND INIT FUNCTION ////////////////////

var _p2pHost host.Host
var _config config.Configuration

// Strategy factory which manages the creation of the strategy instance
var _strategyFactory strategyFactory

// Lock used to manage the singleton strategy instance
var _lock *sync.Mutex

// Singleton instance representing the strategy adopted from the DFaaS agent
var _strategyInstance Strategy

// Initialize initializes this package
func Initialize(p2pHost host.Host, config config.Configuration) {
	_p2pHost = p2pHost
	_config = config
	_lock = &sync.Mutex{}

	switch _config.Strategy {
	case constants.RecalcStrategy:
		_strategyFactory = &recalcStrategyFactory{}
		break
	case constants.NodeMarginStrategy:
		_strategyFactory = &nodeMarginStrategyFactory{}
		break
	}
}

//////////////////// PUBLIC STRUCT TYPES ////////////////////

// Strategy interface represents a generic strategy. 
// Every new strategy for the agent must implement this interface.
type Strategy interface {
	// Method which executes the strategy
	RunStrategy() error
	// Method called when a message is received from a peer
	OnReceived(msg *pubsub.Message) error
}

//////////////////// PUBLIC METHODS ////////////////////

// GetStrategyInstance returns the singleton Strategy instance.
// In a critical section checks if the strategy is already instantiated and returns it.
// If instance is nil, creates a new Strategy instance using the createStrategy method
// of the strategy factory
func GetStrategyInstance() (Strategy, error) {
	var err error

	_lock.Lock()
	defer _lock.Unlock()

	if _strategyInstance == nil {
		_strategyInstance, err = _strategyFactory.createStrategy()
		if err != nil {
			return nil, err
		}
	}

	return _strategyInstance, nil
}