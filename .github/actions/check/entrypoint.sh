#!/bin/sh 
set -e
go mod download
golangci-lint run
go test -cover ./...