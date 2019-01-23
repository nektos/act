#!/bin/sh 

echo "--------------------------" > /dev/stderr
pwd
echo "--------------------------"
ls -lah
echo "--------------------------"
find
echo "--------------------------"
#golangci-lint run
#go test -cover ./...