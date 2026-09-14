# Throwaway Ubuntu 22.04 builder for the .deb when the native build host is unreachable (RELEASES.md, step 7).
FROM --platform=linux/amd64 ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev ca-certificates curl git xz-utils \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://go.dev/dl/go1.26.1.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH
RUN CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16
WORKDIR /src/client
