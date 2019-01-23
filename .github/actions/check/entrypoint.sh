#!/bin/sh 
set -e
golangci-lint run
go test -cover ./...