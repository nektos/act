# choco supports only Mono 5.20.x
# https://github.com/chocolatey/choco#prerequisites
FROM mono:5.20 as builder
# 0.10.15 with newer Mono is failing as it's dependent on Mono Profile 4.0 which has been deprecated
# http://www.mono-project.com/docs/about-mono/releases/4.0.0/#dropped-support-for-old-frameworks
# Alternative to consider, use git repository and reset to specific tag/hash
ARG CHOCOVERSION=0.10.16-beta

ENV DEBIAN_FRONTEND=noninteractive
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
