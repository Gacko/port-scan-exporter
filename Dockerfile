# Start from Alpine 3.14.3.
FROM alpine:3.14.3

# Add binary.
ADD port-scan-exporter /

# Define entrypoint.
ENTRYPOINT ["/port-scan-exporter"]
