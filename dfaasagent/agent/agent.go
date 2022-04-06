package agent

import (
	"context"
	cryptorand "crypto/rand"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/cliflags"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/communication"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/discovery/kademlia"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/discovery/mdns"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logic"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/maddrhelp"
)

//////////////////// PRIVATE VARIABLES ////////////////////

var _p2pHost host.Host

//////////////////// PRIVATE FUNCTIONS ////////////////////

// getPrivateKey loads the libp2p Host's private key from file if it exists and
// it is not empty, or generates it and writes it to the file otherwise
func getPrivateKey(filePath string) (crypto.PrivKey, error) {
	var (
		prvKey crypto.PrivKey = nil
		err    error
		data   []byte
	)

	logger := logging.Logger()

	fileInfo, fileErr := os.Stat(filePath)
	fileExists := fileErr == nil
	keyAlreadyPresent := fileExists && fileInfo.Size() > 0

	if filePath != "" && keyAlreadyPresent {
		logger.Debug("Loading the RSA private key file content from: \"", filePath, "\"")

		data, err = ioutil.ReadFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, "Error while loading the RSA private key file content")
		}

		prvKey, err = crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, errors.Wrap(err, "Error while unmarshalling the RSA private key from file")
		}
	} else {
		logger.Debug("Generating a new RSA key pair")

		prvKey, _, err = crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, cryptorand.Reader)
		if err != nil {
			return nil, errors.Wrap(err, "Error while generating a new RSA key pair")
		}

		if filePath != "" {
			logger.Debug("Writing the newly generated RSA private key to file")

			data, err = crypto.MarshalPrivateKey(prvKey)
			if err != nil {
				return nil, errors.Wrap(err, "Error while marshalling the newly generated RSA private key")
			}

			err = ioutil.WriteFile(filePath, data, 0644)
			if err != nil {
				return nil, errors.Wrap(err, "Error while writing the RSA private key to file")
			}
		}
	}

	return prvKey, nil
}

// runAgent is the main function to be called once we got some very basic setup,
// such as parsed CLI flags and a usable logger
func runAgent(flags *cliflags.ParsedValues) error {
	// Obtain the global logger object
	logger := logging.Logger()

	// Create a new context for libp2p and all the other stuff related to
	// dfaasagent
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	////////// LIBP2P INITIALIZATION //////////

	// RSA key pair for this p2p host
	prvKey, err := getPrivateKey(flags.PrivateKeyFile)
	if err != nil {
		return err
	}

	// Construct a new libp2p Host
	_p2pHost, err = libp2p.New(ctx, libp2p.ListenAddrs(flags.Listen...), libp2p.Identity(prvKey))
	if err != nil {
		return errors.Wrap(err, "Error while creating the libp2p Host")
	}

	// Print this host's multiaddresses
	myMAddrs, err := maddrhelp.BuildHostFullMAddrs(_p2pHost)
	if err != nil {
		return errors.Wrap(err, "Error while building the p2p host's multiaddress")
	}
	logger.Info("Libp2p host started. You can connect to this host by using the following multiaddresses:")
	for i, addr := range myMAddrs {
		logger.Info("  ", i+1, ". ", addr)
	}

	////////// PUBSUB INITIALIZATION //////////

	// The PubSub initialization must be done before the Kademlia one. Otherwise
	// the agent won't be able to publish or subscribe.
	err = communication.Initialize(ctx, _p2pHost, logic.OnReceived)
	if err != nil {
		return err
	}
	logger.Debug("PubSub initialization completed")

	////////// KADEMLIA DHT INITIALIZATION //////////

	// Kademlia and DHT initialization, with connection to bootstrap nodes
	err = kademlia.Initialize(
		ctx,
		_p2pHost,
		flags.BootstrapNodes,
		flags.BootstrapForce,
		flags.Rendezvous,
		flags.KadIdleTime,
	)

	if err != nil {
		return err
	}

	logger.Debug("Connection to Kademlia bootstrap nodes completed")

	////////// mDNS INITIALIZATION //////////

	if flags.MDNSInterval > 0 {
		// mDNS discovery service initialization
		err = mdns.Initialize(ctx, _p2pHost, flags.Rendezvous, flags.MDNSInterval)
		if err != nil {
			return err
		}
		logger.Debug("mDNS discovery service is enabled and initialized")
	}

	////////// LOGIC INITIALIZATION //////////

	err = logic.Initialize(_p2pHost)
	if err != nil {
		return err
	}

	////////// GOROUTINES //////////

	chanStop := make(chan os.Signal, 1)
	signal.Notify(chanStop, syscall.SIGINT, syscall.SIGTERM)

	chanErr := make(chan error, 1)

	go func() { chanErr <- kademlia.RunDiscovery() }()

	if flags.MDNSInterval > 0 {
		go func() { chanErr <- mdns.RunDiscovery() }()
	}

	go func() { chanErr <- communication.RunReceiver() }()

	go func() { chanErr <- logic.RunRecalc() }()

	select {
	case sig := <-chanStop:
		logger.Warn("Caught " + sig.String() + " signal. Stopping.")
		_p2pHost.Close()
		return nil
	case err = <-chanErr:
		_p2pHost.Close()
		return err
	}
}

//////////////////// MAIN FUNCTION ////////////////////

// Main is the main function to be called from outside
func Main() {
	// Initializes Go random number generator
	rand.Seed(int64(time.Now().Nanosecond()))

	// Parse CLI flags
	err := cliflags.ParseOrHelp()
	if err != nil {
		log.Fatal(err)
	}
	flags := cliflags.GetValues()

	// Setup logging engine
	logger, err := logging.Initialize(flags.DateTime, flags.DebugMode, flags.LogColors)
	if err != nil {
		log.Fatal(err)
	}
	logger.Debug("Logger set up successfully")

	// Print the actual CLI flags at DEBUG level (useful for debugging purposes)
	logger.Debug("Parsed CLI flags: ", flags)

	err = runAgent(flags)
	if err != nil {
		logger.Fatal(err)
	}
}
