# Local Linux build environment: `bash build/release-linux.sh` inside it (docs/BUILD.md).
# Keep the apt list in step with the build job of .github/workflows/linux.yml.
FROM ubuntu:22.04
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev ca-certificates curl git xz-utils \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL "https://go.dev/dl/go1.26.1.linux-${TARGETARCH}.tar.gz" | tar -C /usr/local -xz
RUN node_arch=$([ "$TARGETARCH" = amd64 ] && echo x64 || echo "$TARGETARCH") \
    && curl -fsSL "https://nodejs.org/dist/v24.21.0/node-v24.21.0-linux-${node_arch}.tar.xz" \
      | tar -C /usr/local --strip-components=1 -xJ
ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH
RUN CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16
WORKDIR /src/client
