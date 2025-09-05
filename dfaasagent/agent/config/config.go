// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

package config

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/spf13/viper"
)

// Configuration holds the post-processed configuration values.
type Configuration struct {
	DebugMode bool `mapstructure:"AGENT_DEBUG"`
	DateTime  bool `mapstructure:"AGENT_LOG_DATETIME"`
	LogColors bool `mapstructure:"AGENT_LOG_COLORS"`

	// Address where to listen new peers. Should be in form
	// "/ip4/192.0.2.0/tcp/6000". Suggested default is "/ip4/0.0.0.0/tcp/31600".
	Listen []string `mapstructure:"AGENT_LISTEN"`

	// File where the agent's private key can be found. The private key is the
	// ID of the agent for other peers. If not given, a new one will be
	// generated.
	PrivateKeyFile string `mapstructure:"AGENT_PRIVATE_KEY_FILE"`

	// Where to use bootstrap nodes to found other nodes.
	BootstrapNodes bool `mapstructure:"AGENT_BOOTSTRAP_NODES"`

	// If true, use public peers to found other nodes. If false,
	// AGENT_BOOTSTRAP_NODES_LIST or AGENT_BOOTSTRAP_NODES_FILE should be
	// provided.
	PublicBootstrapNodes bool `mapstructure:"AGENT_PUBLIC_BOOTSTRAP_NODES"`

	// List of bootstrap nodes addresses.
	BootstrapNodesList []string `mapstructure:"AGENT_BOOTSTRAP_NODES_LIST"`

	// Path to a file containing bootstrap node information.
	BootstrapNodesFile string `mapstructure:"AGENT_BOOTSTRAP_NODES_FILE"`

	// If true, agent's initialization fails if any bootstrap peer cannot be
	// contacted.
	BootstrapForce bool `mapstructure:"AGENT_BOOTSTRAP_FORCE"`

	// Unique string used for grouping peers for discovery.
	Rendezvous string `mapstructure:"AGENT_RENDEZVOUS"`

	// Set to true to mDNS discovery service to find other nodes.
	MDNSEnabled bool `mapstructure:"AGENT_MDNS_ENABLED"`

	KadIdleTime time.Duration `mapstructure:"AGENT_KAD_IDLE_TIME"`

	PubSubTopic string `mapstructure:"AGENT_PUBSUB_TOPIC"`

	RecalcPeriod time.Duration `mapstructure:"AGENT_RECALC_PERIOD"`

	HAProxyConfigFile string `mapstructure:"AGENT_HAPROXY_CONFIG_FILE"`
	HAProxyHost       string `mapstructure:"AGENT_HAPROXY_HOST"`
	HAProxyPort       uint   `mapstructure:"AGENT_HAPROXY_PORT"`
	HAProxySockPath   string `mapstructure:"AGENT_HAPROXY_SOCK_PATH"`

	OpenFaaSHost string `mapstructure:"AGENT_OPENFAAS_HOST"`
	OpenFaaSPort uint   `mapstructure:"AGENT_OPENFAAS_PORT"`
	OpenFaaSUser string `mapstructure:"AGENT_OPENFAAS_USER"`
	OpenFaaSPass string `mapstructure:"AGENT_OPENFAAS_PASS"`

	Strategy string `mapstructure:"AGENT_STRATEGY"`

	GroupListFileName string `mapstructure:"AGENT_GROUP_LIST_FILE_NAME"`

	NodeType int `mapstructure:"AGENT_NODE_TYPE"`

	CPUThresholdNMS   float64 `mapstructure:"AGENT_NMS_CPU_THRESHOLD"`
	RAMThresholdNMS   float64 `mapstructure:"AGENT_NMS_RAM_THRESHOLD"`
	PowerThresholdNMS float64 `mapstructure:"AGENT_NMS_POWER_THRESHOLD"`
}

// viperBindConfig binds each field of the Configuration struct with its
// corresponding environment variable.
//
// This is necessary because of a bug in the Viper library. See viper's bug
// [188] for more information.
//
// [188]: https://github.com/spf13/viper/issues/188#issuecomment-1273983955
func viperBindConfig() {
	var cfg Configuration

	t := reflect.TypeOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			continue // Skip field without mapstructure tag.
		}
		// Bind the environment variable.
		_ = viper.BindEnv(tag, tag)
	}
}

// LoadConfig reads configuration from environment variables first, and then
// optionally overwrites with a .env file specified by the --config command line
// argument.
func LoadConfig() (config Configuration, err error) {
	viperBindConfig()

	// Parse command line arguments.
	help := flag.Bool("help", false, "Show help message")
	configPath := flag.String("config", "", "Path to .env file to overwrite env vars")
	flag.Parse()

	if *help {
		fmt.Println("Usage: [--config config.env] [--help]")
		fmt.Println("If --config is provided, values from the file will overwrite environment variables.")
		os.Exit(0)
	}

	viper.AllowEmptyEnv(true)

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
