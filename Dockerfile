# syntax=docker/dockerfile:1

# Build the admin app on the builder's own platform.
FROM --platform=$BUILDPLATFORM node:26-slim AS frontend
RUN npm install -g pnpm@12.2.1
WORKDIR /app
COPY pnpm-workspace.yaml pnpm-lock.yaml package.json ./
COPY frontend ./frontend
COPY sdk/frontend ./sdk/frontend
COPY sdk/astro/package.json ./sdk/astro/package.json
COPY plugins ./plugins
COPY test/e2e/package.json ./test/e2e/package.json
COPY test/theme/package.json ./test/theme/package.json
RUN pnpm install --frozen-lockfile
RUN pnpm --filter @gophenberg/frontend build

# Build the server with the admin inside, for the image's platform.
FROM --platform=$BUILDPLATFORM golang:1.27 AS backend
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY sdk/go.mod ./sdk/go.mod
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags embedweb -trimpath -buildvcs=false -ldflags="-s -w" \
	-o /gophenberg ./cmd/gophenberg

# Make the folders the image hands to the nonroot user.
FROM --platform=$BUILDPLATFORM gcr.io/distroless/static-debian12:debug AS folders
RUN ["/busybox/mkdir", "-p", "/themes", "/media"]

# What every image holds: the node a theme runs on, the notices and the settings.
FROM gcr.io/distroless/nodejs24-debian12:nonroot AS base
COPY LICENSE NOTICE /usr/share/doc/gophenberg/
COPY LICENSES /usr/share/doc/gophenberg/LICENSES
COPY --from=folders --chown=65532:65532 /themes /themes
COPY --from=folders --chown=65532:65532 /media /media
ENV GOPHENBERG_THEMES_DIR=/themes
ENV GOPHENBERG_MEDIA_DIR=/media
ENV GOPHENBERG_ADDR=0.0.0.0:8081
ENV GOPHENBERG_NODE_BIN=/nodejs/bin/node
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/gophenberg"]
CMD ["serve"]

# The release image: the very binary the release archives hold.
FROM base AS release
ARG TARGETARCH
COPY --chmod=0755 --from=binaries linux_${TARGETARCH}/gophenberg /gophenberg

# The image built from source, the default target.
FROM base AS runtime
COPY --from=backend /gophenberg /gophenberg
