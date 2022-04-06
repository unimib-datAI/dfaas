package config

import (
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/mitchellh/mapstructure"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/utils/maddrhelp"
	"strings"
	"time"
)

// BootstrapNodes is a multiaddr list with a mapstructure decode hook
// that decodes a comma separated string list.
type BootstrapNodes []multiaddr.Multiaddr

// Listen is a string list with a mapstructure decode hook that decode a bootstrap nodes string.
// Can be one of the following
// - inline comma-separated list:             "list:/ip4/1.2.3.4/...,/ip4/..."
// - txt file path (newline-separated list):  "file:./bootstrap.txt"
// - libp2p public DHT bootstrap peers list:  "public"
// - no bootstrap nodes list specified:       "none"
type Listen []multiaddr.Multiaddr

func (s *Listen) UnmarshalText(text []byte) error {
	*s, _ = maddrhelp.ParseMAddrComma(string(text))
	return nil
}

func (s *BootstrapNodes) UnmarshalText(text []byte) error {
	nodes := string(text)
	var err error
	if nodes == "public" {
		// Use libp2p public bootstrap peers list
		*s = dht.DefaultBootstrapPeers
	} else if strings.HasPrefix(nodes, "list:") {
		list := strings.TrimPrefix(nodes, "list:")
		*s, err = maddrhelp.ParseMAddrComma(list)
		if err != nil {
			return errors.Wrap(err, "Error while parsing bootstrap peers list from string")
		}
	} else if strings.HasPrefix(nodes, "file:") {
		filepath := strings.TrimPrefix(nodes, "file:")
		*s, err = maddrhelp.ParseMAddrComma(filepath)
		if err != nil {
			return errors.Wrap(err, "Error while parsing bootstrap peers list from file")
		}
	} else if nodes != "none" {
		return errors.New("Invalid bootstrap peers list. Please check if the prefix is correct")
	}
	return nil
}

// Configuration holds the post-processed configuration values
type Configuration struct {
	DebugMode bool `mapstructure:"AGENT_DEBUG"`
	DateTime  bool `mapstructure:"AGENT_LOG_DATETIME"`
	LogColors bool `mapstructure:"AGENT_LOG_COLORS"`

	Listen         Listen `mapstructure:"AGENT_LISTEN"`
	PrivateKeyFile string `mapstructure:"AGENT_PRIVATE_KEY_FILE"`

	BootstrapNodes BootstrapNodes `mapstructure:"AGENT_BOOTSTRAP_NODES"`
	BootstrapForce bool           `mapstructure:"AGENT_BOOTSTRAP_FORCE"`
	Rendezvous     string         `mapstructure:"AGENT_RENDEZVOUS"`
	MDNSInterval   time.Duration  `mapstructure:"AGENT_MDNS_INTERVAL"`
	KadIdleTime    time.Duration  `mapstructure:"AGENT_KAD_IDLE_TIME"`
	PubSubTopic    string         `mapstructure:"AGENT_PUBSUB_TOPIC"`

	RecalcPeriod time.Duration `mapstructure:"AGENT_RECALC_PERIOD"`

	HAPRoxyTemplateFile        string `mapstructure:"AGENT_HAPROXY_TEMPLATE_FILE"`
	HAProxyConfigFile          string `mapstructure:"AGENT_HAPROXY_CONFIG_FILE"`
	HAProxyConfigUpdateCommand string `mapstructure:"AGENT_HAPROXY_CONFIG_UPDATE_COMMAND"`
	HAProxyHost                string `mapstructure:"AGENT_HAPROXY_HOST"`
	HAProxyPort                uint   `mapstructure:"AGENT_HAPROXY_PORT"`
	HAProxySockPath            string `mapstructure:"AGENT_HA_SOCK_PATH"`

	OpenFaaSHost string `mapstructure:"AGENT_OPENFAAS_HOST"`
	OpenFaaSPort uint   `mapstructure:"AGENT_OPENFAAS_PORT"`
	OpenFaaSUser string `mapstructure:"AGENT_OPENFAAS_USER"`
	OpenFaaSPass string `mapstructure:"AGENT_OPENFAAS_PASS"`

	PrometheusHost string `mapstructure:"AGENT_PROMETHEUS_HOST"`
	PrometheusPort uint   `mapstructure:"AGENT_PROMETHEUS_PORT"`
}

func LoadConfig(path string) (config Configuration, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("dfaasagent")
	viper.SetConfigType("env")
	viper.SetEnvPrefix("AGENT")
	viper.AllowEmptyEnv(true)

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	viper.AutomaticEnv()

	viper.Debug()

	err = viper.Unmarshal(&config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.TextUnmarshallerHookFunc(),
	)))
	return
}
