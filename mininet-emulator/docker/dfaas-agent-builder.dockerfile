FROM golang:1.14 as build

# Copy all the necessary files needed to build the agent
COPY dfaasagent /go/src/dfaasagent
WORKDIR /go/src/dfaasagent

# Build the agent only if it was not already built outside of the container. In
# this case, use the already present executable file (to accelerate testing)
RUN if [ ! -f "./dfaasagent" ]; \
    then \
        go build; \
    else \
        echo "Using already build agent executable"; \
    fi