FROM nestybox/ubuntu-focal-systemd:latest

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

RUN git clone https://github.com/openfaas/faasd --depth=1 /faasd

WORKDIR /faasd

# COPY files/faasd/cmd/install.go ./cmd/install.go

COPY files/faasd/hack/install.sh ./hack/install.sh
RUN chmod +x ./hack/install.sh
RUN ./hack/install.sh

RUN echo "admin" > /var/lib/faasd/secrets/basic-auth-password

RUN systemctl enable faasd.service
RUN systemctl enable faasd-provider.service

### Platform (OpenFaaS - faasd)

### Agent
WORKDIR /agent
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/dfaasagent .
COPY files/dfaasagent/haproxycfg.tmpl ./haproxycfg.tmpl
COPY files/dfaasagent/dfaasagent.service /etc/systemd/system/dfaasagent.service

RUN systemctl enable dfaasagent.service
### End Agent

WORKDIR /

# Set systemd as entrypoint.
ENTRYPOINT [ "/sbin/init", "--log-level=err" ]