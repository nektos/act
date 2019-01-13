FROM golang:1.11.4-stretch

RUN go get -u honnef.co/go/tools/cmd/staticcheck
RUN go get -u golang.org/x/lint/golint
RUN go get -u github.com/fzipp/gocyclo

COPY "entrypoint.sh" "/entrypoint.sh"
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
