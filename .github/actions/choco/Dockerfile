FROM alpine:latest

ARG CHOCOVERSION=0.11.3

RUN apk add --no-cache bash ca-certificates \
  && apk --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing add mono-dev \
  && cert-sync /etc/ssl/certs/ca-certificates.crt \
  && wget "https://github.com/chocolatey/choco/archive/${CHOCOVERSION}.tar.gz" \
  && tar -xzf "${CHOCOVERSION}.tar.gz" \
  && mv "choco-${CHOCOVERSION}" /opt/chocolatey \
  && chmod +x build.sh zip.sh \
  && ./build.sh -v \
  && ln -sf /opt /opt/chocolatey/opt \
  && mkdir -p /opt/chocolatey/lib \
  && apk del ca-certificates \
  && rm -rf /var/cache/apk/*

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
