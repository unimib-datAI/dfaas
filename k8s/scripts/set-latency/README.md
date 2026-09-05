# set-latency

This is a small Python CLI utility designed to configure artificial network
latency, jitter, and packet loss between Linux VMs. The VMs and their network
links are defined in a YAML configuration file.

Run the script with `./set-latency.py --help` for more information.

## How to run

The target VMs must be accessible through SSH without an interactive password
and the SSH user must have passwordless `sudo` access. The VMs must also have
`tc` and `ping` installed.

You need to have `uv` installed to be able to run the script. It is better to
run the script on a Linux host external to the target VMs.

Set the latency configuration with:

```console
$ ./set-latency --config config.yaml apply
```

To remove the configuration:

```console
$ ./set-latency --config config.yaml clear
```

To inspect the current configuration:

```console
$ ./set-latency --config config.yaml status
```

To test all configured links:

```console
$ ./set-latency --config config.yaml test
```

See the given example configuration `config_example.yaml` for more information.
