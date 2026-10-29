FROM alpine:3.21@sha256:ee16d92f96db08d9d2d07da48555a27f3a69d8f0759021dbc601195ed0d2b5bd

ARG CHOCOVERSION=1.1.0
# sha256 of https://github.com/chocolatey/choco/archive/${CHOCOVERSION}.tar.gz. Update both together.
ARG CHOCOSHA256=331f85e220c09824ea77aeb01020e1f182da712dddeb066415af9c2115fb384f

RUN apk add --no-cache bash ca-certificates git \
  && apk --no-cache --repository https://dl-cdn.alpinelinux.org/alpine/edge/community add mono mono-dev \
  && cert-sync /etc/ssl/certs/ca-certificates.crt \
  && wget "https://github.com/chocolatey/choco/archive/${CHOCOVERSION}.tar.gz" -O /tmp/choco.tar.gz \
  && echo "${CHOCOSHA256}  /tmp/choco.tar.gz" | sha256sum -c - \
  && tar -xzf /tmp/choco.tar.gz \
  && rm /tmp/choco.tar.gz \
  && cd choco-"${CHOCOVERSION}" \
  && chmod +x build.sh zip.sh \
  && ./build.sh -v \
  && mv ./code_drop/chocolatey/console /opt/chocolatey \
  && mkdir -p /opt/chocolatey/lib \
  && rm -rf /choco-"${CHOCOVERSION}" \
  && apk del mono-dev \
  && rm -rf /var/cache/apk/*

ENV ChocolateyInstall=/opt/chocolatey
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
