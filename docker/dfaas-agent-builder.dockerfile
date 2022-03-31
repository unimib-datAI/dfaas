FROM golang:1.14 as build

COPY ./ /go/src/dfaasagent
WORKDIR /go/src/dfaasagent

RUN go build