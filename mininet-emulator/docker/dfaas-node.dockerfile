FROM nestybox/ubuntu-impish-systemd:latest

### Proxy (HAProxy)

RUN apt-get update && apt-get install -y haproxy \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN systemctl enable haproxy.service

### End Proxy (HAProxy)

### Platform (OpenFaaS - faasd)
RUN apt-get update && apt-get install -y \
    libc6 \
    libc6-dev \
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

# Add cAdvisor to monitor containers
RUN wget https://github.com/google/cadvisor/releases/download/v0.39.3/cadvisor
RUN chmod +x cadvisor
COPY files/faasd/cadvisor.service /etc/systemd/system/cadvisor.service
RUN systemctl enable cadvisor.service

# Add Prometheus node exporter to monitor node
RUN wget https://github.com/prometheus/node_exporter/releases/download/v1.3.1/node_exporter-1.3.1.linux-amd64.tar.gz
RUN tar xvfz node_exporter-1.3.1.linux-amd64.tar.gz && rm node_exporter-1.3.1.linux-amd64.tar.gz
COPY files/faasd/node-exporter.service /etc/systemd/system/node-exporter.service
RUN systemctl enable node-exporter.service

# Override Prometheus configuration
COPY files/faasd/prometheus.yml /var/lib/faasd/prometheus.yml

### Platform (OpenFaaS - faasd)

### Agent
WORKDIR /agent
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/dfaasagent .
COPY files/dfaasagent/haproxycfg.tmpl ./haproxycfg.tmpl
COPY files/dfaasagent/dfaasagent.service /etc/systemd/system/dfaasagent.service
COPY files/dfaasagent/init-functions.json ./init-functions.json

RUN systemctl enable dfaasagent.service
### End Agent

WORKDIR /

# Set systemd as entrypoint.
ENTRYPOINT [ "/sbin/init", "--log-level=err" ]