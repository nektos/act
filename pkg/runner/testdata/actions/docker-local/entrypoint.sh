#!/bin/sh -l

echo "Hello $1"
time=$(date)
echo ::set-output name=time::$time
echo ::set-output name=whoami::$WHOAMI

echo "SOMEVAR=$1" >>$GITHUB_ENV
