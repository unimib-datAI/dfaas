##### Build stage #####

FROM golang:1.14 as build

# Copy all the necessary files needed to build the agent
COPY . /go/src/dfaasagent
WORKDIR /go/src/dfaasagent

# Build the agent only if it was not already built outside of the container. In
# this case, use the already present executable file (to accelerate testing)
RUN if [ ! -f "./dfaasagent" ]; \
    then \
        go build; \
    else \
        echo "Using already build agent executable"; \
    fi

##### Production-ready stage #####
FROM ubuntu:bionic

RUN apt-get update && apt-get install -y \
    net-tools \
    iputils-ping \
    iproute2

# Docker installation (to be able to use the docker command inside the
# container)
RUN apt-get update && apt-get install -y \
    wget \
    net-tools \
    iputils-ping \
    iproute2 && \
    wget -O docker.tgz "https://download.docker.com/linux/static/stable/x86_64/docker-19.03.12.tgz" && \
    tar --extract \
        --file docker.tgz \
        --strip-components 1 \
        --directory /usr/local/bin/ && \
    rm docker.tgz && \
    apt-get clean && rm -rf /var/lib/apt/lists/* && \
    docker --version

# Copy the necessary files into the production stage
WORKDIR /agent
COPY --from=build /go/src/dfaasagent/dfaasagent .
COPY haproxycfg.tmpl .

# Main DFaaS Docker volume
VOLUME /dfaasvolume

CMD /bin/bash

# ENTRYPOINT ["/agent/dfaasagent"]