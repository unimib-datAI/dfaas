FROM haproxy:lts-alpine

USER root

RUN apk add --update --no-cache \
        bash \
        tcpdump \
        iperf \
        busybox-extras \
        iproute2 \
        iputils

USER haproxy

CMD /bin/bash
# CMD ["haproxy", "-f", "/usr/local/etc/haproxy/haproxy.cfg"]