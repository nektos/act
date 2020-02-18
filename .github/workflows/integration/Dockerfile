FROM golangci/golangci-lint:v1.23.6

COPY "entrypoint.sh" "/entrypoint.sh"
RUN chmod +x /entrypoint.sh

ENV GOFLAGS -mod=vendor
ENTRYPOINT ["/entrypoint.sh"]
