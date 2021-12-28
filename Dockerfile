FROM alpine:3.14.3

ADD port-scan-exporter /

ENTRYPOINT ["/port-scan-exporter"]