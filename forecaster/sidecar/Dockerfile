FROM alpine:3.20

RUN apk add --no-cache jq curl

WORKDIR /sidecar
COPY sidecar.sh /sidecar/sidecar.sh
RUN chmod +x /sidecar/sidecar.sh

ENTRYPOINT ["/sidecar/sidecar.sh"]
