# Dockerfile based on the official Go image: https://hub.docker.com/_/golang
FROM golang:1.22

WORKDIR /opt/dfaasagent

# Cache the dependencies.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -v ./...

CMD ["/opt/dfaasagent/dfaasagent"]
