# Megumi Code / mgm CLI container image.
#
# The CLI is a single static (CGO-free) binary; the final image is a minimal
# Alpine with ca-certificates (for HTTPS to the broker) and a shell (for
# interactive `docker run -it`). Multi-arch via buildx TARGET* args.
#
#   docker build --build-arg VERSION=v0.1.0 -t mgmlaboratory/mgm .
#   docker run --rm mgmlaboratory/mgm version

# --- build stage -----------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags "-s -w -X github.com/MGM-Laboratory/mgm-cli/internal/version.Version=${VERSION}" \
      -o /out/mgm ./cmd/mgm

# --- runtime stage ---------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 megumi
COPY --from=build /out/mgm /usr/local/bin/mgm
USER megumi
ENV DO_NOT_TRACK=1
ENTRYPOINT ["mgm"]
CMD ["--help"]
