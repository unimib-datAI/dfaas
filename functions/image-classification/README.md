# image-classification OpenFaaS function

This folder contains the source code for the `image-classification` function in
OpenFaaS. It implements a basic Python function for image classification using
Torchvision.

The function file structure is primarily derived from the
[python3-http-debian](https://github.com/openfaas/python-flask-template/tree/master/template/python3-http-debian)
template in the [OpenFaaS Python Flask
templates](https://github.com/openfaas/python-flask-template). Some files have
been removed, and the Dockerfile has been simplified. The main function logic
can be found in [function/handler.py](function/handler.py).

## Build and push

Use the help script to build the image from the project's root directory:

```console
$ ./k8s/scripts/build-image.sh openfaas-image-classification none --dockerfile functions/image-classification/Dockerfile
```

This will generate the image with `localhost/openfaas-image-classification:dev`
tag. To push the image to GitHub Container Registry (after login) with tag
`ghcr.io/unimib-datai/dfaas-openfaas-image-classification`:

```console
$ ./k8s/scripts/build-image.sh openfaas-image-classification push --dockerfile functions/image-classification/Dockerfile
```

> [!IMPORTANT]
> OpenFaaS CE does not support private images. A workaround is to publish the
> images to a public registry. See [this
> comment](https://github.com/unimib-datAI/dfaas/issues/32#issuecomment-3356311152)
> for more details.

Use the public image to deploy the function to OpenFaaS:

```console
$ faas-cli deploy --image=ghcr.io/unimib-datai/dfaas-openfaas-image-classification:dev --name=mlimage --label dfaas.maxrate=100
```

> [!IMPORTANT]
> Do not use hyphens or underscores, otherwise they will break the HAProxy
> generated configuration!

This will deploy the function to the local OpenFaaS instance as `mlimage`. The
`dfaas.maxrate` label is required by the Recalc strategy. You can omit the label
if the DFaaS Agent does not use this strategy.

## Test

Download example images from the ImageNet dataset. In this case, we use a few
samples from
[github.com/EliSchwartz/imagenet-sample-images](https://github.com/EliSchwartz/imagenet-sample-images):

```console
$ wget https://github.com/EliSchwartz/imagenet-sample-images/blob/master/n01770393_scorpion.JPEG?raw=true -O scorpion.jpeg
$ wget https://github.com/EliSchwartz/imagenet-sample-images/blob/master/n01616318_vulture.JPEG?raw=true -O vulture.jpeg
$ wget https://github.com/EliSchwartz/imagenet-sample-images/blob/master/n01770393_scorpion.JPEG?raw=true -O scorpion.jpeg
```

Then, use curl to send the requests:

```console
$ curl -X POST --data-binary "@path/file.jpg" -H "Content-Type: image/jpeg" http://127.0.0.1:30080/function/mlimage
# Example output:
content-length: 58
content-type: application/json
date: Wed, 01 Oct 2025 14:15:56 GMT
server: waitress
x-call-id: 16eef834-f5d2-4b73-ac62-506fcc3e598a
x-duration-seconds: 0.286761
x-served-by: openfaas-ce/0.27.12
x-start-time: 1759328156681799394
x-server: IP

[{"class": "scorpion", "probability": 0.9953627586364746}]
```

Note that the endpoint is http://127.0.0.1:30080, the HAProxy public port, and
not directly the OpenFaaS Gateway. The IP address should be adjusted to match
your local OpenFaaS instance.
