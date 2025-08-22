#!/bin/bash
set -e

# Print a greeting
echo "Hello, $1!"

# Print all environment variables
echo "Environment Variables:"
env

ls -la "$PWD"
ls -la "$PWD/docker-action-host-env"

if [ -f "$PWD/docker-action-host-env/Dockerfile" ]; then
  echo "Dockerfile exists in workspace."
else
  echo "Dockerfile does not exist in workspace."
fi
