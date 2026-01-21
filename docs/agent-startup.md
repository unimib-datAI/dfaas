# Agent Startup

Each agent is assigned a unique Peer Identity (Peer ID), which is an important
component for connecting other agents within the network. The Peer ID is derived
from an ed25519 keypair. Specifically it is a multihash of the public key (with
the public key itself derived from the corresponding private key). For more
details, see the [official libp2p
documentation](https://docs.libp2p.io/concepts/fundamentals/peers).

By default, when you start a DFaaS agent, it automatically generates a new Peer
ID and prints both the Peer ID and the private key in PEM format to standard
output. Note that the private key is not saved anywhere. As a result, each time
you restart the agent, a new keypair (and thus a new Peer ID) will be created.
To persist the same Peer ID across restarts, set the `AGENT_PRIVATE_KEY_FILE`
environment variable to the file path containing your private key in PEM format
when starting the agent.

## Manual Key Creation

To manually create an ed25519 keypair, you can use OpenSSL to save the key as a
PEM file:

```console
$ openssl genpkey -algorithm ED25519 -out private_key.pem
```

Once you have your private key file, provide its path to the
`AGENT_PRIVATE_KEY_FILE` environment variable when launching the agent.

You may also want to determine the Peer ID corresponding to your private key so
that other agents can connect to your node. Note that the Peer ID derivation
from a private key does not follow a standardized method. To obtain it, you can
use a custom Go tool included at `k8s/scripts/libp2p-peer-id`:

```console
$ cd k8s/scripts/libp2p-peer-id
$ go build .
$ ./libp2p-peer-id PRIVATE_KEY_FILE.pem
```

Here, `PRIVATE_KEY_FILE.pem` refers to your PEM encoded private key.

> [!IMPORTANT]
> You must have the Go toolchain installed to build and run the `libp2p-peer-id`
> program!

For additional information on key derivation and Peer IDs, refer to the
[official libp2p
specification](https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md#peer-ids).

## How to Configure the Agent

You can configure the agent using environment variables. A complete list of
supported variables is available in the [agent's source
code](https://github.com/unimib-datAI/dfaas/blob/main/dfaasagent/agent/config/config.go).
If you are deploying the agent via the Helm chart, it's recommended to configure
it by creating a custom `values.yaml` file that overrides the [default
values](https://github.com/unimib-datAI/dfaas/blob/main/k8s/charts/agent/values.yaml).
Throughout this document, examples refer to modifying the `values.yaml` file.

## Agent Connection

DFaaS agents can be connected in several ways. The most straightforward approach
is to provide the multiaddr of one agent to others. A multiaddr contains all
information necessary to connect to another agent: the network protocol (IPv4 is
assumed), the IP address, the transport protocol (assumed TCP), the port
(typically 31600), and the Peer ID. In practice, you mainly need the agent's IP
address and Peer ID. For more about multiaddrs, see the [official
specification](https://github.com/multiformats/multiaddr), and for details on
how libp2p utilizes multiaddrs, refer to the [libp2p
specification](https://github.com/libp2p/specs/tree/master/addressing). This
section explains how to configure the DFaaS agent to connect to peers.

To have the agent automatically connect to other agents at startup, set the
`AGENT_BOOTSTRAP_NODES` environment variable to `true` and
`AGENT_PUBLIC_BOOTSTRAP_NODES` to `false`. This instructs the agent to connect
to peers listed in the `AGENT_BOOTSTRAP_NODES_LIST` environment variable, which
should be a comma-separated list of multiaddrs.

By default, the agent will continue its startup process even if it encounters
errors while connecting to the nodes in the list. To force the agent to retry
connecting until all specified nodes are reached, set the
`AGENT_BOOTSTRAP_FORCE` environment variable to `true`.

Below is an example configuration in a `values.yaml` file:

```yaml
config:
  AGENT_BOOTSTRAP_NODES: true
  AGENT_PUBLIC_BOOTSTRAP_NODES: false
  AGENT_BOOTSTRAP_NODES_LIST: "/ip4/10.12.68.5/tcp/31600/p2p/12D3KooWQUp1rDNQuWn7QHNm2oye356xkwWriqgbgzBbjuMcoM13,/ip4/10.12.68.4/tcp/31600/p2p/12D3KooWS48D4dvPgbVCU347L2sVZN65m1u5nnJDJs1erqGW3p53"
  AGENT_BOOTSTRAP_FORCE: true

privateKey: |
  -----BEGIN PRIVATE KEY-----
  ...your key here...
  -----END PRIVATE KEY-----
```

In this example, the agent will attempt to connect at startup to two peer agents
and will keep retrying until both connections are successfully established.
