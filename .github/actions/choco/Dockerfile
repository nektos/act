  
FROM mono:3.12.1 as builder
ARG CHOCOVERSION=0.10.15

RUN echo "deb http://archive.debian.org/debian/ wheezy main contrib non-free" >/etc/apt/sources.list
RUN apt-get update && apt-get install -y wget tar gzip

WORKDIR /usr/local/src
RUN wget "https://github.com/chocolatey/choco/archive/${CHOCOVERSION}.tar.gz"
RUN tar -xzf "${CHOCOVERSION}.tar.gz"
RUN mv "choco-${CHOCOVERSION}" choco

WORKDIR /usr/local/src/choco
RUN chmod +x build.sh zip.sh
RUN ./build.sh -v

FROM alpine:latest

COPY --from=builder /usr/local/src/choco/build_output/chocolatey /opt/chocolatey

RUN apk add --no-cache bash
RUN apk --update --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing add mono-dev \
  && apk --update --no-cache add -t build-dependencies ca-certificates \
  && cert-sync /etc/ssl/certs/ca-certificates.crt \
  && ln -sf /opt /opt/chocolatey/opt \
  && mkdir -p /opt/chocolatey/lib \
  && apk del build-dependencies \
  && rm -rf /var/cache/apk/*


COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]


