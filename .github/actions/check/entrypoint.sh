#!/bin/sh 

golangci-lint run
go test -cover ./...