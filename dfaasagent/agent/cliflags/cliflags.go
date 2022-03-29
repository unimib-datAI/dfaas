package cliflags

import (
	"fmt"
	"os"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/maddrhelp"
)

// This package is for parsing dfaasagent's CLI flags

// We use the "github.com/spf13/pflag" library instead of the default Go "flag"
// library, because we want POSIX/GNU-style --flags

type rawValues struct {
	Listen         string
	PrivateKeyFile string

	BootstrapNodes string
	BootstrapForce bool
	Rendezvous     string
	MDNSInterval   string
	KadIdleTime    string

	DebugMode bool
	DateTime  bool
	LogColors bool

    //InitFunctionsFile string
	RecalcPeriod string

	HATemplateFile        string
	HAConfigFile          string
	HAConfigUpdateCommand string
	HAProxyHost           string
	HAProxyPort           uint
	HASockPath            string

	OpenFaaSHost   string
	OpenFaaSPort   uint
	OpenFaaSUser   string
	OpenFaaSPass   string
	PrometheusHost string
	PrometheusPort uint
}

// ParsedValues holds the post-processed flags values
type ParsedValues struct {
	Listen         []multiaddr.Multiaddr
	PrivateKeyFile string

	BootstrapNodes []multiaddr.Multiaddr
	BootstrapForce bool
	Rendezvous     string
	MDNSInterval   time.Duration
	KadIdleTime    time.Duration

	DebugMode bool
	DateTime  bool
	LogColors bool

    // InitFunctionsFile string
	RecalcPeriod time.Duration

	HATemplateFile        string
	HAConfigFile          string
	HAConfigUpdateCommand string
	HAProxyHost           string
	HAProxyPort           uint
	HASockPath            string

	OpenFaaSHost   string
	OpenFaaSPort   uint
	OpenFaaSUser   string
	OpenFaaSPass   string
	PrometheusHost string
	PrometheusPort uint
}

func parseRawValues() *rawValues {
	valsRaw := &rawValues{}

	descrBootstrap := "Bootstrap nodes address list. Can be one of the following:\n"
	descrBootstrap += "  - inline comma-separated list:             \"list:/ip4/1.2.3.4/...,/ip4/...\"\n"
	descrBootstrap += "  - txt file path (newline-separated list):  \"file:./bootstrap.txt\"\n"
	descrBootstrap += "  - libp2p public DHT bootstrap peers list:  \"public\"\n"
	descrBootstrap += "  - no bootstrap nodes list specified:       \"none\""

	pflag.StringVarP(&valsRaw.Listen, "listen", "l", "/ip4/0.0.0.0/tcp/0", "Listening addresses (comma-separated multiaddress list)")
	pflag.StringVar(&valsRaw.PrivateKeyFile, "prvkey", "", "Path to the libp2p private key file. If empty, a newly generated random key will be used. If not empty but the file doesn't exist, then a random key is written to the file and then used.")

	pflag.StringVarP(&valsRaw.BootstrapNodes, "bootstrap-nodes", "b", "none", descrBootstrap)
	pflag.BoolVarP(&valsRaw.BootstrapForce, "bootstrap-force", "f", false, "If true, dfaasagent fails if it cannot connect to all bootstrap nodes")
	pflag.StringVarP(&valsRaw.Rendezvous, "rendezvous", "r", "dfaasagent", "Univoque string to identify a group of nodes. It must be the same on every node that should be interconnected")
	pflag.StringVarP(&valsRaw.MDNSInterval, "mdns-interval", "m", "0s", "MDNS discovery interval. If <= 0, mDNS discovery will be disabled")
	pflag.StringVarP(&valsRaw.KadIdleTime, "kad-idle", "k", "10s", "Kademlia discovery idle time. Should be > 0")

	pflag.BoolVarP(&valsRaw.DebugMode, "debug", "v", false, "Enable debug/verbose mode")
	pflag.BoolVarP(&valsRaw.DateTime, "datetime", "t", false, "Enable date and time in logging")
	pflag.BoolVarP(&valsRaw.LogColors, "colors", "c", false, "Enable colored logging levels")

    // pflag.StringVar(&valsRaw.InitFunctionsFile, "init-functions", "./init-functions.json", "Path to the init-functions.json file")
	pflag.StringVar(&valsRaw.RecalcPeriod, "recalc-period", "5m", "This determines every how much to execute the recalculation function across the p2p network (publish rate limits, recalc weights, etc...). Should be > 0. It is advised to not go below 10s, to avoid problems with Prometheus")

	pflag.StringVar(&valsRaw.HATemplateFile, "hatemplate", "haproxycfg.tmpl", "HAProxy configuration template file path. It must be readable")
	pflag.StringVar(&valsRaw.HAConfigFile, "haconfig", "haproxy.cfg", "HAProxy configuration file path. It must be writable")
	pflag.StringVarP(&valsRaw.HAConfigUpdateCommand, "haconfig-update-command", "u", ":", "Command which will be executed after updating the HAProxy configuration file. The default value (\":\") is the Bash no-op command")
	pflag.StringVar(&valsRaw.HAProxyHost, "haproxy-host", "127.0.0.1", "IP address or hostname of the local HAProxy instance. Should be reachable from the network since it will be used by the other nodes")
	pflag.UintVar(&valsRaw.HAProxyPort, "haproxy-port", 80, "TCP port number of the local HAProxy instance. Should be reachable from the network since it will be used by the other nodes")
	pflag.StringVar(&valsRaw.HASockPath, "hasock-path", "unix:///run/haproxy/admin.sock", "Path to the HAProxy socket. Supported address schemas are tcp://hostname:port and unix:///path/to/file")

	pflag.StringVar(&valsRaw.OpenFaaSHost, "openfaas-host", "127.0.0.1", "IP address or hostname of the local OpenFaaS instance. Should be reachable at least by the local HAProxy instance")
	pflag.UintVar(&valsRaw.OpenFaaSPort, "openfaas-port", 8080, "TCP port number of the local OpenFaaS instance. Should be reachable at least by the local HAProxy instance")
	pflag.StringVar(&valsRaw.OpenFaaSUser, "openfaas-user", "admin", "Administration username for local OpenFaaS instance")
	pflag.StringVar(&valsRaw.OpenFaaSPass, "openfaas-pass", "admin", "Administration password for local OpenFaaS instance")
	pflag.StringVar(&valsRaw.PrometheusHost, "prometheus-host", "127.0.0.1", "IP address or hostname of the Prometheus instance for gathering OpenFaaS metrics. Should be reachable by this agent")
	pflag.UintVar(&valsRaw.PrometheusPort, "prometheus-port", 9090, "TCP port number of the Prometheus instance for gathering OpenFaaS metrics. Should be reachable by this agent")

	pflag.Parse()

	return valsRaw
}

func convertRawToParsed(valsRaw *rawValues) (*ParsedValues, error) {
	var err error
	valsParsed := &ParsedValues{}

	valsParsed.Listen, err = maddrhelp.ParseMAddrComma(valsRaw.Listen)
	if err != nil {
		return nil, errors.Wrap(err, "Error while parsing listen addresses list")
	}

	valsParsed.PrivateKeyFile = valsRaw.PrivateKeyFile

	if valsRaw.BootstrapNodes == "public" {
		// Use libp2p public bootstrap peers list
		valsParsed.BootstrapNodes = dht.DefaultBootstrapPeers
	} else if strings.HasPrefix(valsRaw.BootstrapNodes, "list:") {
		list := strings.TrimPrefix(valsRaw.BootstrapNodes, "list:")
		valsParsed.BootstrapNodes, err = maddrhelp.ParseMAddrComma(list)
		if err != nil {
			return nil, errors.Wrap(err, "Error while parsing bootstrap peers list from string")
		}
	} else if strings.HasPrefix(valsRaw.BootstrapNodes, "file:") {
		filepath := strings.TrimPrefix(valsRaw.BootstrapNodes, "file:")
		valsParsed.BootstrapNodes, err = maddrhelp.ParseMAddrFile(filepath)
		if err != nil {
			return nil, errors.Wrap(err, "Error while parsing bootstrap peers list from file")
		}
	} else if valsRaw.BootstrapNodes != "none" {
		return nil, errors.New("Invalid bootstrap peers list. Please check if the prefix is correct")
	}

	valsParsed.BootstrapForce = valsRaw.BootstrapForce
	valsParsed.Rendezvous = valsRaw.Rendezvous

	valsParsed.MDNSInterval, err = time.ParseDuration(valsRaw.MDNSInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Error while parsing MDNSInterval")
	}
	if valsParsed.MDNSInterval < 0 {
		valsParsed.MDNSInterval = 0
	}

	valsParsed.KadIdleTime, err = time.ParseDuration(valsRaw.KadIdleTime)
	if err != nil {
		return nil, errors.Wrap(err, "Error while parsing KadIdleTime")
	}
	if valsParsed.KadIdleTime <= 0 {
		return nil, errors.New("Invalid KadIdleTime value. Should be > 0")
	}

	valsParsed.DebugMode = valsRaw.DebugMode

	valsParsed.DateTime = valsRaw.DateTime
	valsParsed.LogColors = valsRaw.LogColors

    // valsParsed.InitFunctionsFile = valsRaw.InitFunctionsFile
	valsParsed.RecalcPeriod, err = time.ParseDuration(valsRaw.RecalcPeriod)
	if err != nil {
		return nil, errors.Wrap(err, "Error while parsing RecalcPeriod")
	}
	if valsParsed.RecalcPeriod <= 0 {
		return nil, errors.New("Invalid RecalcPeriod value. Should be > 0")
	}

	valsParsed.HATemplateFile = valsRaw.HATemplateFile
	valsParsed.HAConfigFile = valsRaw.HAConfigFile
	valsParsed.HAConfigUpdateCommand = valsRaw.HAConfigUpdateCommand
	valsParsed.HAProxyHost = valsRaw.HAProxyHost
	valsParsed.HAProxyPort = valsRaw.HAProxyPort
	valsParsed.HASockPath = valsRaw.HASockPath

	valsParsed.OpenFaaSHost = valsRaw.OpenFaaSHost
	valsParsed.OpenFaaSPort = valsRaw.OpenFaaSPort
	valsParsed.OpenFaaSUser = valsRaw.OpenFaaSUser
	valsParsed.OpenFaaSPass = valsRaw.OpenFaaSPass
	valsParsed.PrometheusHost = valsRaw.PrometheusHost
	valsParsed.PrometheusPort = valsRaw.PrometheusPort

	return valsParsed, nil
}

var publicParsedValues *ParsedValues

// ParseOrHelp parses CLI flags and returns the resulting configuration
func ParseOrHelp() error {
	var showHelp bool
	pflag.BoolVarP(&showHelp, "help", "h", false, "Shows the help message")

	// We need to execute this before calling pflag.Usage() to get the parameter
	// names, descriptions, etc...
	valsRaw := parseRawValues()

	// If the "--help" flag was specified, display the help message and exit
	if showHelp {
		fmt.Println("DFaaS Agent")
		fmt.Println()
		pflag.Usage()
		os.Exit(0)
	}

	var err error
	publicParsedValues, err = convertRawToParsed(valsRaw)
	if err != nil {
		return err
	}

	return nil
}

// GetValues returns the parsed values. You should have called ParseOrHelp
// before calling this function, otherwise it will return nil
func GetValues() *ParsedValues {
	return publicParsedValues
}
