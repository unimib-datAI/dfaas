FROM ubuntu:22.04 as runtime
WORKDIR app

RUN apt-get update \
    && apt update \
    && apt install libssl3 \
    && DEBIAN_FRONTEND="noninteractive" apt-get install -y ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY ./scaphandre /usr/local/bin
ENTRYPOINT ["/usr/local/bin/scaphandre"]
