FROM haproxytech/haproxy-ubuntu

RUN apt-get update && apt-get install -y \
    wget \
    net-tools \
    iputils-ping \
    iproute2 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

CMD /bin/bash

# ENTRYPOINT ["/docker-entrypoint.sh"]
# CMD ["haproxy", "-f", "/usr/local/etc/haproxy/haproxy.cfg"]