FROM alpine:latest

RUN apk update && apk add \
    python3 \
    py3-pip \
    curl \
    wget \
    bash \
    jq

RUN wget https://github.com/tsenart/vegeta/releases/download/v12.8.4/vegeta_12.8.4_linux_amd64.tar.gz
RUN tar -xf vegeta_12.8.4_linux_amd64.tar.gz && rm vegeta_12.8.4_linux_amd64.tar.gz
RUN mv vegeta /usr/local/bin/

WORKDIR /
COPY files/plot-requirements.txt ./plot-requirements.txt
COPY files/plot-results.py ./plot-results.py
RUN pip3 install --break-system-packages -r plot-requirements.txt
RUN chmod +x plot-results.py

COPY files/operator_entrypoint.sh ./operator_entrypoint.sh
RUN chmod +x operator_entrypoint.sh
ENTRYPOINT [ "./operator_entrypoint.sh" ]