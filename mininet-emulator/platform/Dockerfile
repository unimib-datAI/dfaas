FROM docker:dind

RUN apk add --update --no-cache \
        curl \
        git \
        bash \
        tcpdump \
        iperf \
        busybox-extras \
        iproute2 \
        iputils \
        py-pip \
        python3-dev \
        libffi-dev \
        openssl-dev \
        gcc \
        libc-dev \
        make \
        rust \
        cargo

RUN pip install docker-compose

RUN git clone https://github.com/openfaas/faasd --depth=1

ENTRYPOINT ["dockerd-entrypoint.sh"]

CMD /bin/bash