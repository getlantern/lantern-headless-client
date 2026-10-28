FROM debian:12-slim
ARG TARGETPLATFORM
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN update-ca-certificates
COPY $TARGETPLATFORM/lantern-headless /lantern-headless
ENTRYPOINT ["/lantern-headless"]
