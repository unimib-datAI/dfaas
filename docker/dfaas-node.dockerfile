FROM nestybox/ubuntu-jammy-systemd:latest@sha256:93b72540b784f16276396780418851c9d39f3392132fe8bf2988733002d9dd24

### Proxy (HAProxy)

RUN apt-get update && apt-get install -y haproxy python3.11 python3-pip \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN systemctl enable haproxy.service

### End Proxy (HAProxy)

### Platform (OpenFaaS - faasd)
RUN apt-get update && apt-get install -y \
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

RUN git clone --branch 0.18.6 https://github.com/openfaas/faasd /faasd

WORKDIR /faasd

COPY files/faasd/hack/install.sh ./hack/install.sh
RUN chmod +x ./hack/install.sh
RUN ./hack/install.sh

RUN echo "admin" > /var/lib/faasd/secrets/basic-auth-password

RUN systemctl enable faasd.service
RUN systemctl enable faasd-provider.service

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

# Override Prometheus configuration
COPY files/faasd/prometheus.yml /var/lib/faasd/prometheus.yml

### Platform (OpenFaaS - faasd)

WORKDIR /
COPY files/deploy_functions.sh ./deploy_functions.sh
RUN chmod +x deploy_functions.sh
COPY files/entrypoint.sh ./entrypoint.sh
RUN chmod +x entrypoint.sh

### DFaaS Forecaster
WORKDIR /forecaster
COPY forecaster/ .
RUN pip install "fastapi[all]" scikit-learn lightgbm joblib pandas numpy
COPY files/forecaster/forecaster.service /etc/systemd/system/forecaster.service
RUN systemctl enable forecaster.service
### End DFaaS Forecaster

### Agent
WORKDIR /agent
COPY files/dfaasagent/dfaasagent.service /etc/systemd/system/dfaasagent.service
RUN systemctl enable dfaasagent.service
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/agent/logic/haproxycfgrecalc.tmpl ./haproxycfgrecalc.tmpl
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/agent/logic/haproxycfgnms.tmpl ./haproxycfgnms.tmpl
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/agent/groupsreader/group_list.json .
COPY --from=dfaas-agent-builder:latest /go/src/dfaasagent/dfaasagent ./dfaasagent
### End Agent

WORKDIR /
ENTRYPOINT [ "./entrypoint.sh" ]
