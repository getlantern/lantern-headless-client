FROM debian:11-slim
COPY lantern-headless /lantern-headless
ENTRYPOINT ["/lantern-headless"]
