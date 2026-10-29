# Materialize a fixture image into the local docker store via buildx --load.
# The GitHub Actions cache (type=gha) keeps the layers between runs.
# IMAGE is the tag under test (see cache-fixtures.sh), so it cannot carry a digest here.
ARG IMAGE
FROM ${IMAGE}
