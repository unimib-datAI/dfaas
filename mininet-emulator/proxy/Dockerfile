FROM haproxy:lts-alpine

RUN apk add --update --no-cache \
        bash \
        tcpdump \
        iperf \
        busybox-extras \
        iproute2 \
        iputils \

CMD /bin/bash
# CMD ["haproxy", "-f", "/usr/local/etc/haproxy/haproxy.cfg"]