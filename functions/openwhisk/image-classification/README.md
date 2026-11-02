# image-classification OpenWhisk function

This folder contains the source code for the `image-classification` function in
OpenWhisk. It implements a basic Python Web Action for image classification
using Torchvision.

This is the OpenWhisk port of the [OpenFaaS
version](../../image-classification). The main function logic is mostly the
same. The differences are:

* [\_\_main\_\_.py](__main__.py) contains the `main(args)` entry point called by
  the OpenWhisk Python runtime. It reads the Web Action parameters, calls
  `classify()` from `function.py` and returns a Web Action response.
* The [Containerfile](Containerfile) extends the official
  `openwhisk/action-python-v3.12` runtime image and only installs the
  dependencies, since PyTorch is too large for a zip action. The action code is
  is given when creating the action.

## Build and push

Use the project's [justfile](../../../justfile) to build the image:

```console
$ just build openwhisk-image-classification functions/openwhisk/image-classification
```

This will generate the image with `localhost/openwhisk-image-classification:dev`
tag. To push the image to GitHub Container Registry (after login) with tag
`ghcr.io/unimib-datai/dfaas-openwhisk-image-classification`:

```console
$ just build openwhisk-image-classification functions/openwhisk/image-classification push
```

## Deploy

Package the action code in a zip file:

```console
$ cd functions/openwhisk/image-classification
$ zip action.zip __main__.py function.py
```

We assume the `wsk` CLI is configured with the administrator credentials (see
[the OpenWhisk chart README](../../../k8s/charts/openwhisk/README.md)), so the
action is created in the `whisk.system` namespace. Since the namespace is given
by the credentials, there is no need to specify it.

OpenWhisk pulls the runtime image from a public registry. Create the action as a
Web Action using the custom runtime image and the zip file:

```console
$ wsk action create mlimage action.zip \
    --docker ghcr.io/unimib-datai/dfaas-openwhisk-image-classification:dev \
    --web true --memory 256 --timeout 120000 --annotation dfaas.maxrate 100
```

Note that DFaaS Agent considers only Web Actions. You can tune `--memory` to the
size that matches the model weights, 256 should be enough. The `dfaas.maxrate`
annotation is required by the Recalc strategy. You can omit it if the DFaaS
Agent does not use this strategy. You can also set `dfaas.timeout_ms` to
configure the per-request timeout, or `timeout` for the Web Action timeout.

To update only the code, rebuild the zip file and run `wsk action update` with
the same arguments.

## Test

Download example images from the ImageNet dataset. In this case, we use a few
samples from
[github.com/EliSchwartz/imagenet-sample-images](https://github.com/EliSchwartz/imagenet-sample-images):

```console
$ wget https://github.com/EliSchwartz/imagenet-sample-images/blob/master/n01770393_scorpion.JPEG?raw=true -O scorpion.jpeg
$ wget https://github.com/EliSchwartz/imagenet-sample-images/blob/master/n01616318_vulture.JPEG?raw=true -O vulture.jpeg
```

Then, use curl to send the requests (the action is in the `default` package of
the `whisk.system` namespace):

```console
$ curl -X POST --data-binary "@scorpion.jpeg" -H "Content-Type: image/jpeg" http://127.0.0.1:30080/api/v1/web/whisk.system/default/mlimage
[{"class": "scorpion", "probability": 0.9953627586364746}]
```

Note that the endpoint is http://127.0.0.1:30080, the HAProxy public port, and
not directly the OpenWhisk API Gateway. The IP address should be adjusted to
match your local OpenWhisk instance.
