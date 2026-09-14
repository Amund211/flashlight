# Dockerfile for the sidecar OTEL Collector
# Based on: https://cloud.google.com/run/docs/tutorials/custom-metrics-opentelemetry-sidecar
# https://github.com/GoogleCloudPlatform/opentelemetry-operations-collector
FROM us-docker.pkg.dev/cloud-ops-agents-artifacts/google-cloud-opentelemetry-collector/otelcol-google:0.156.1@sha256:0b028b4d76119e23c984b2b25e2137b6a16ae4ff54ae1d6ba444d6733174e955

COPY config.yaml /etc/otelcol-google/config.yaml
