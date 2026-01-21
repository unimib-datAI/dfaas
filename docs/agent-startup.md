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
