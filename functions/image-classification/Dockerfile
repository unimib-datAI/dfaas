# This Dockerfile is based on the official OpenFaaS Python Flask template,
# adapted from python3-http-debian.
#
# See: https://github.com/openfaas/python-flask-template
ARG PYTHON_VERSION=3.12
ARG DEBIAN_OS=slim

FROM --platform=${TARGETPLATFORM:-linux/amd64} ghcr.io/openfaas/of-watchdog:0.11.4 AS watchdog
FROM --platform=${TARGETPLATFORM:-linux/amd64} python:${PYTHON_VERSION}-${DEBIAN_OS} AS build

COPY --from=watchdog /fwatchdog /usr/bin/fwatchdog
RUN chmod +x /usr/bin/fwatchdog

ARG UPGRADE_PACKAGES=false

RUN apt-get update -qy \
    && if [ "${UPGRADE_PACKAGES}" = "true" ] || [ "${UPGRADE_PACKAGES}" = "1" ]; then apt-get upgrade -qy; fi \
    && apt-get install -qy --no-install-recommends gcc make ${ADDITIONAL_PACKAGE} \
    && rm -rf /var/lib/apt/lists/*

# Add non root user
RUN addgroup --system app \
    && adduser app --system --ingroup app  --home /home/app \
    && chown app:app /home/app

USER app

ENV PATH=$PATH:/home/app/.local/bin

WORKDIR /opt/app

COPY functions/image-classification/main.py .
COPY functions/image-classification/function.py .

COPY functions/image-classification/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY functions/image-classification/requirements.torch.txt .
RUN pip install --no-cache-dir --user -r requirements.torch.txt

FROM build AS ship
WORKDIR /opt/app

# Configure of-watchdog in HTTP mode.
# See: https://github.com/openfaas/of-watchdog?tab=readme-ov-file#modes
ENV mode="http"
ENV fprocess="python main.py"
ENV cgi_headers="true"
ENV upstream_url="http://127.0.0.1:5000"

CMD ["fwatchdog"]
