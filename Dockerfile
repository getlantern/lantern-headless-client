FROM debian:11-slim
ARG TARGETPLATFORM
RUN apt update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN update-ca-certificates
COPY $TARGETPLATFORM/lantern-headless /lantern-headless
ENTRYPOINT ["/lantern-headless"]
