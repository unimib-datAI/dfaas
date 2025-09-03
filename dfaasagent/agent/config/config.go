// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Configuration holds the post-processed configuration values.
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
	// TODO: IT IS BASE32!
	OpenFaaSPass string `mapstructure:"AGENT_OPENFAAS_PASS"`

	Strategy string `mapstructure:"AGENT_STRATEGY"`

	GroupListFileName string `mapstructure:"AGENT_GROUP_LIST_FILE_NAME"`

	NodeType int `mapstructure:"AGENT_NODE_TYPE"`

	CPUThresholdNMS   float64 `mapstructure:"AGENT_NMS_CPU_THRESHOLD"`
	RAMThresholdNMS   float64 `mapstructure:"AGENT_NMS_RAM_THRESHOLD"`
	PowerThresholdNMS float64 `mapstructure:"AGENT_NMS_POWER_THRESHOLD"`
}

// LoadConfig reads configuration from environment variables first, and then
// optionally overwrites with a .env file specified by the --config command line
// argument.
func LoadConfig() (config Configuration, err error) {
	// Parse command line arguments.
	help := flag.Bool("help", false, "Show help message")
	configPath := flag.String("config", "", "Path to .env file to overwrite env vars")
	flag.Parse()

	if *help {
		fmt.Println("Usage: [--config config.env] [--help]")
		fmt.Println("If --config is provided, values from the file will overwrite environment variables.")
		os.Exit(0)
	}

	// Read env variables.
	viper.SetEnvPrefix("AGENT")
	viper.AllowEmptyEnv(true)
	viper.AutomaticEnv()

	// If --config is provided and the file exists, load it and overwrite env
	// vars.
	if *configPath != "" {
		if _, statErr := os.Stat(*configPath); statErr == nil {
			viper.SetConfigFile(*configPath)
			viper.SetConfigType("env")

			// Only overwrite values from the file.
			readErr := viper.ReadInConfig()
			if readErr != nil {
				err = readErr
				return
			}
		} else if !os.IsNotExist(statErr) {
			// If error is not "file does not exist", return statErr
			err = statErr
			return
		}
	}

	err = viper.Unmarshal(&config)
	return
}
