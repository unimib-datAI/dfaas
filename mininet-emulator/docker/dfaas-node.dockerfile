FROM nestybox/ubuntu-focal-systemd:latest

### Agent
WORKDIR /agent
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/dfaasagent .
COPY files/dfaasagent/haproxycfg.tmpl ./haproxycfg.tmpl
COPY files/dfaasagent/dfaasagent.service /etc/systemd/system/dfaasagent.service

RUN systemctl enable dfaasagent.service
### End Agent

### Proxy (HAProxy)

RUN apt-get update && apt-get install -y haproxy \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN systemctl enable haproxy.service

### End Proxy (HAProxy)

### Platform (OpenFaaS - faasd)
RUN apt-get update && apt-get install -y \
    curl \
    git \
    wget \
    net-tools \
    iputils-ping \
    iproute2 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/openfaas/faasd --depth=1

WORKDIR /faasd

COPY files/faasd/hack/install.sh ./hack/install.sh
RUN chmod +x ./hack/install.sh

COPY files/faasd/cmd/install.go ./cmd/install.go

RUN ./hack/install.sh
# RUN systemctl enable faasd.service
# RUN systemctl enable faasd-provider.service

### Platform (OpenFaaS - faasd)

WORKDIR /

# Set systemd as entrypoint.
ENTRYPOINT [ "/sbin/init", "--log-level=err" ]