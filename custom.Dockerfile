# Use a small base image
FROM scratch
# Copy the Traefik binary from the build context into the container
COPY script/ca-certificates.crt /etc/ssl/certs/
COPY dist/traefik /
EXPOSE 80 443 8080
VOLUME ["/tmp"]
# Run Traefik with no arguments; you can specify arguments when you run the container
ENTRYPOINT ["/usr/local/bin/traefik"]
