FROM debian:11-slim
RUN apt update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN update-ca-certificates
COPY lantern-headless /lantern-headless
ENTRYPOINT ["/lantern-headless"]
