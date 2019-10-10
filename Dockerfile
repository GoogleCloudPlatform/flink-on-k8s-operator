# Docker image for the Flink Operator.
#
# This image relies on the flink-operator-builder image to be available locally,
# as it copies the flink-operator binary from the builder image.

# Use distroless as minimal base image to package the Flink Operator binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=flink-operator-builder /workspace/bin/flink-operator .
ENTRYPOINT ["/flink-operator"]
