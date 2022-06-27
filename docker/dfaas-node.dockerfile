FROM ghcr.io/nestybox/ubuntu-impish-systemd-docker:latest

### Proxy (HAProxy)

RUN apt-get update && apt-get install -y haproxy \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN systemctl enable haproxy.service

### End Proxy (HAProxy)

### Platform (OpenFaaS)
RUN curl -SLsf https://get.arkade.dev/ | sh
RUN curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg
RUN echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
RUN apt-get update && apt-get install -y \
    kubectl \
    nano \
    libc6 \
    libc6-dev \
    curl \
    git \
    wget \
    net-tools \
    iputils-ping \
    iproute2 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Add K3s
RUN wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.4.3 bash

# Add faas-cli
RUN arkade get faas-cli

# Add cAdvisor to monitor containers
RUN wget https://github.com/google/cadvisor/releases/download/v0.44.0/cadvisor
RUN chmod +x cadvisor
COPY files/faasd/cadvisor.service /etc/systemd/system/cadvisor.service
RUN systemctl enable cadvisor.service

# Add Prometheus node exporter to monitor node
RUN wget https://github.com/prometheus/node_exporter/releases/download/v1.3.1/node_exporter-1.3.1.linux-amd64.tar.gz
RUN tar xvfz node_exporter-1.3.1.linux-amd64.tar.gz && rm node_exporter-1.3.1.linux-amd64.tar.gz
COPY files/faasd/node-exporter.service /etc/systemd/system/node-exporter.service
RUN systemctl enable node-exporter.service

### End Platform (OpenFaaS)

WORKDIR /
COPY files/deploy_functions.sh ./deploy_functions.sh
RUN chmod +x deploy_functions.sh
COPY files/entrypoint.sh ./entrypoint.sh
RUN chmod +x entrypoint.sh

### Agent
WORKDIR /agent
COPY files/dfaasagent/dfaasagent.service /etc/systemd/system/dfaasagent.service
RUN systemctl enable dfaasagent.service
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/haproxycfg.tmpl ./haproxycfg.tmpl
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/dfaasagent ./dfaasagent
### End Agent

WORKDIR /
ENTRYPOINT [ "./entrypoint.sh" ]