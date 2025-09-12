# This Dockerfile build the operator.
#
# The operator is a Bash script that calls vegeta and a Python script to
# generate plots.

# Use this layer just to download vegeta.
FROM alpine AS downloader

# See: https://docs.github.com/en/packages/learn-github-packages/connecting-a-repository-to-a-package#connecting-a-repository-to-a-container-image-using-the-command-line
LABEL org.opencontainers.image.source="https://github.com/unimib-datAI/dfaas"
LABEL org.opencontainers.image.description="DFaaS Operator with vegeta ${VEGETA_VERSION}"
LABEL org.opencontainers.image.licenses="AGPL-3.0"

ARG VEGETA_VERSION=12.12.0

RUN apk add --no-cache wget

RUN wget https://github.com/tsenart/vegeta/releases/download/v${VEGETA_VERSION}/vegeta_${VEGETA_VERSION}_linux_amd64.tar.gz \
    && tar -xf vegeta_${VEGETA_VERSION}_linux_amd64.tar.gz

FROM python:3.12-alpine

ARG VEGETA_VERSION=12.8.4

RUN apk add --no-cache curl bash jq

COPY --from=downloader vegeta /usr/local/bin/

WORKDIR /

COPY operator/plot-requirements.txt operator/plot-results.py ./

RUN pip install --break-system-packages --requirement plot-requirements.txt

RUN chmod +x plot-results.py

COPY operator/operator.sh ./
RUN chmod +x operator.sh

ENTRYPOINT [ "./operator.sh" ]
