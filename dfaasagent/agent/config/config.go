// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package config

import (
	"time"

	"github.com/spf13/viper"
)

// Configuration holds the post-processed configuration values
type Configuration struct {
	DebugMode bool `mapstructure:"AGENT_DEBUG"`
	DateTime  bool `mapstructure:"AGENT_LOG_DATETIME"`
	LogColors bool `mapstructure:"AGENT_LOG_COLORS"`

	Listen         []string `mapstructure:"AGENT_LISTEN"`
	PrivateKeyFile string   `mapstructure:"AGENT_PRIVATE_KEY_FILE"`

	BootstrapNodes       bool     `mapstructure:"AGENT_BOOTSTRAP_NODES"`
	PublicBootstrapNodes bool     `mapstructure:"AGENT_PUBLIC_BOOTSTRAP_NODES"`
	BootstrapNodesList   []string `mapstructure:"AGENT_BOOTSTRAP_NODES_LIST"`
	BootstrapNodesFile   string   `mapstructure:"AGENT_BOOTSTRAP_NODES_FILE"`
	BootstrapForce       bool     `mapstructure:"AGENT_BOOTSTRAP_FORCE"`

	Rendezvous   string        `mapstructure:"AGENT_RENDEZVOUS"`
	MDNSInterval time.Duration `mapstructure:"AGENT_MDNS_INTERVAL"`
	KadIdleTime  time.Duration `mapstructure:"AGENT_KAD_IDLE_TIME"`
	PubSubTopic  string        `mapstructure:"AGENT_PUBSUB_TOPIC"`

	RecalcPeriod time.Duration `mapstructure:"AGENT_RECALC_PERIOD"`

	HAProxyConfigFile string `mapstructure:"AGENT_HAPROXY_CONFIG_FILE"`
	HAProxyHost       string `mapstructure:"AGENT_HAPROXY_HOST"`
	HAProxyPort       uint   `mapstructure:"AGENT_HAPROXY_PORT"`
	HAProxySockPath   string `mapstructure:"AGENT_HAPROXY_SOCK_PATH"`

	OpenFaaSHost string `mapstructure:"AGENT_OPENFAAS_HOST"`
	OpenFaaSPort uint   `mapstructure:"AGENT_OPENFAAS_PORT"`
	OpenFaaSUser string `mapstructure:"AGENT_OPENFAAS_USER"`
	OpenFaaSPass string `mapstructure:"AGENT_OPENFAAS_PASS"`

	PrometheusHost string `mapstructure:"AGENT_PROMETHEUS_HOST"`
	PrometheusPort uint   `mapstructure:"AGENT_PROMETHEUS_PORT"`

	HttpServerHost string `mapstructure:"AGENT_HTTPSERVER_HOST"`
	HttpServerPort uint   `mapstructure:"AGENT_HTTPSERVER_PORT"`

	ForecasterHost string `mapstructure:"AGENT_FORECASTER_HOST"`
	ForecasterPort uint   `mapstructure:"AGENT_FORECASTER_PORT"`

	Strategy string `mapstructure:"AGENT_STRATEGY"`

	GroupListFileName string `mapstructure:"AGENT_GROUP_LIST_FILE_NAME"`

	NodeType int `mapstructure:"AGENT_NODE_TYPE"`

	CPUThresholdNMS   float64 `mapstructure:"AGENT_NMS_CPU_THRESHOLD"`
	RAMThresholdNMS   float64 `mapstructure:"AGENT_NMS_RAM_THRESHOLD"`
	PowerThresholdNMS float64 `mapstructure:"AGENT_NMS_POWER_THRESHOLD"`
}

func LoadConfig(configPath string) (config Configuration, err error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("env")
	viper.SetEnvPrefix("AGENT")
	viper.AllowEmptyEnv(true)

	// Override values in config file with env vars.
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
