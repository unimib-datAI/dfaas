package config

import (
	"github.com/spf13/viper"
	"time"
)

type BootstrapConfiguration struct {
	BootstrapNodes      bool     `mapstructure:"AGENT_BOOTSTRAP_NODES"`
	PublicBoostrapNodes bool     `mapstructure:"AGENT_PUBLIC_BOOTSTRAP_NODES"`
	BootstrapNodesList  []string `mapstructure:"AGENT_BOOTSTRAP_NODES_LIST"`
	BootstrapNodesFile  string   `mapstructure:"AGENT_BOOTSTRAP_NODES_FILE"`
	BootstrapForce      bool     `mapstructure:"AGENT_BOOTSTRAP_FORCE"`
}

// Configuration holds the post-processed configuration values
type Configuration struct {
	DebugMode bool `mapstructure:"AGENT_DEBUG"`
	DateTime  bool `mapstructure:"AGENT_LOG_DATETIME"`
	LogColors bool `mapstructure:"AGENT_LOG_COLORS"`

	Listen         []string `mapstructure:"AGENT_LISTEN"`
	PrivateKeyFile string   `mapstructure:"AGENT_PRIVATE_KEY_FILE"`

	BoostrapConfig BootstrapConfiguration
	Rendezvous     string        `mapstructure:"AGENT_RENDEZVOUS"`
	MDNSInterval   time.Duration `mapstructure:"AGENT_MDNS_INTERVAL"`
	KadIdleTime    time.Duration `mapstructure:"AGENT_KAD_IDLE_TIME"`
	PubSubTopic    string        `mapstructure:"AGENT_PUBSUB_TOPIC"`

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

	err = viper.Unmarshal(&config)
	return
}
