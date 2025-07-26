FROM debian:bullseye-slim

# Install dependencies
RUN apt-get update && apt-get install -y \
    curl \
    && apt-get clean

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["/entrypoint.sh"]