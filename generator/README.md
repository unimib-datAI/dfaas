# Generator component

In this module there are 2 file:

- _configurations.py_: contains configurtions of each node type (RAM and CPU) and associated max_rates and minimun and maximum deployable number of replicas.

- _wl-generator.py_: this script generate, using configurations for different node types, random scenarios to simulate. This configurations are exported under _experiments_ directory.

**Note**: before execute _wl-generator.py_ script change _node_ variable with the name of the node that want to generate configuration to. By default it generates 10 configuration.

Example of usage:

```console
python wl-generator.py 
```