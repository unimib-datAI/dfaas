// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright 2021-2025 The DFaaS Authors. All rights reserved.
// This file is licensed under the AGPL v3.0 or later license. See LICENSE and
// AUTHORS file for more information.

module gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent

go 1.21.1

toolchain go1.22.2

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/bcicen/go-haproxy v0.0.0-20180203142132-ff5824fe38be
	github.com/libp2p/go-libp2p v0.9.2
	github.com/libp2p/go-libp2p-core v0.5.6
	github.com/libp2p/go-libp2p-discovery v0.4.0
	github.com/libp2p/go-libp2p-kad-dht v0.7.12
	github.com/libp2p/go-libp2p-pubsub v0.3.0
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.17.0
)

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/btcsuite/btcd v0.20.1-beta // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20190912175916-7055855a373f // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gocarina/gocsv v0.0.0-20200925213129-04be9ee2e1a2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gopacket v1.1.17 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/huin/goupnp v1.0.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/ipfs/go-cid v0.0.5 // indirect
	github.com/ipfs/go-datastore v0.4.4 // indirect
	github.com/ipfs/go-ipfs-util v0.0.1 // indirect
	github.com/ipfs/go-ipns v0.0.2 // indirect
	github.com/ipfs/go-log v1.0.4 // indirect
	github.com/ipfs/go-log/v2 v2.0.8 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/koron/go-ssdp v0.0.0-20191105050749-2e1c40ed0b5d // indirect
	github.com/libp2p/go-addr-util v0.0.2 // indirect
	github.com/libp2p/go-buffer-pool v0.0.2 // indirect
	github.com/libp2p/go-conn-security-multistream v0.2.0 // indirect
	github.com/libp2p/go-eventbus v0.1.0 // indirect
	github.com/libp2p/go-flow-metrics v0.0.3 // indirect
	github.com/libp2p/go-libp2p-autonat v0.2.3 // indirect
	github.com/libp2p/go-libp2p-blankhost v0.1.6 // indirect
	github.com/libp2p/go-libp2p-circuit v0.2.2 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.4.2 // indirect
	github.com/libp2p/go-libp2p-loggables v0.1.0 // indirect
	github.com/libp2p/go-libp2p-mplex v0.2.3 // indirect
	github.com/libp2p/go-libp2p-nat v0.0.6 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.2.4 // indirect
	github.com/libp2p/go-libp2p-pnet v0.2.0 // indirect
	github.com/libp2p/go-libp2p-record v0.1.2 // indirect
	github.com/libp2p/go-libp2p-secio v0.2.2 // indirect
	github.com/libp2p/go-libp2p-swarm v0.2.4 // indirect
	github.com/libp2p/go-libp2p-tls v0.1.3 // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.3.0 // indirect
	github.com/libp2p/go-libp2p-yamux v0.2.7 // indirect
	github.com/libp2p/go-mplex v0.1.2 // indirect
	github.com/libp2p/go-msgio v0.0.4 // indirect
	github.com/libp2p/go-nat v0.0.5 // indirect
	github.com/libp2p/go-netroute v0.1.2 // indirect
	github.com/libp2p/go-openssl v0.0.5 // indirect
	github.com/libp2p/go-reuseport v0.0.1 // indirect
	github.com/libp2p/go-reuseport-transport v0.0.3 // indirect
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/libp2p/go-stream-muxer-multistream v0.3.0 // indirect
	github.com/libp2p/go-tcp-transport v0.2.0 // indirect
	github.com/libp2p/go-ws-transport v0.3.1 // indirect
	github.com/libp2p/go-yamux v1.3.7 // indirect
	github.com/miekg/dns v1.1.41 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/minio/sha256-simd v0.1.1 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.0 // indirect
	github.com/mr-tron/base58 v1.1.3 // indirect
	github.com/multiformats/go-base32 v0.0.3 // indirect
	github.com/multiformats/go-multiaddr-dns v0.2.0 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multiaddr-net v0.1.5 // indirect
	github.com/multiformats/go-multibase v0.0.2 // indirect
	github.com/multiformats/go-multihash v0.0.13 // indirect
	github.com/multiformats/go-multistream v0.1.1 // indirect
	github.com/multiformats/go-varint v0.0.5 // indirect
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spacemonkeygo/spacelog v0.0.0-20180420211403-2296661a0572 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/mdns v0.0.0-20190826153040-b9b60ed33aa9 // indirect
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7 // indirect
	github.com/whyrusleeping/timecache v0.0.0-20160911033111-cfcb2f1abfee // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
