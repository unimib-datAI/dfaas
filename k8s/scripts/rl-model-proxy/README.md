WIP

Example mapping:

```console
$ cat map.json
{
    "node_XXX": "node_0",
    "node_YYY": "node_1",
    "node_ZZZ": "node_2",
    "node_JJJ": "node_3",
    "node_KKK": "node_4"
}
```

Example observation input:

```console
$ cat body_test_custom_nodes.json
{
    "observation": {
        "node_XXX": {
            "input_rate": 4,
            "previous_input_rate": 2,
            "previous_fwd_to_node_1": 1,
            "previous_fwd_to_node_2": 1,
            [...]
        }
    }
}
```

Example invocation with `curl`:

```console
$ curl -s "http://localhost:8080/action" -d @body_test_custom_nodes.json -H "Content-Type: application/json"
{
    "node_XXX": {
        "local":0.9999946355819702,
        "node_YYY":9.961642035705154e-7,
        "node_ZZZ":0.0000012219852578709833,
        "node_JJJ":0.0000010784980304379133,
        "node_KKK":0.0000012444977528502932,
        "reject":9.146793331638037e-7
    }
}
```
