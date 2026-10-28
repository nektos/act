# Materialize a fixture image into the local docker store via buildx --load.
# The GitHub Actions cache (type=gha) keeps the layers between runs.
ARG IMAGE
FROM ${IMAGE}
