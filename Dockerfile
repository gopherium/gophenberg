# syntax=docker/dockerfile:1

# Build the single-page app.
FROM node:26-slim AS frontend
RUN npm install -g pnpm@11.10.0
WORKDIR /app
COPY pnpm-workspace.yaml pnpm-lock.yaml package.json ./
COPY patches ./patches
COPY frontend ./frontend
COPY sdk/frontend ./sdk/frontend
COPY sdk/astro/package.json ./sdk/astro/package.json
COPY plugins ./plugins
COPY test/e2e/package.json ./test/e2e/package.json
COPY test/theme/package.json ./test/theme/package.json
RUN pnpm install --frozen-lockfile
RUN pnpm --filter @gophenberg/frontend build

# Build the statically linked server binary.
FROM golang:1.27 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gophenberg ./cmd/gophenberg
RUN mkdir -p /themes /media

# Assemble the runtime image: the binary, the built SPA, and the node a theme is served with.
FROM gcr.io/distroless/nodejs24-debian12:nonroot
COPY --from=backend /gophenberg /gophenberg
COPY --from=frontend /app/frontend/dist /web
COPY --from=backend --chown=65532:65532 /themes /themes
COPY --from=backend --chown=65532:65532 /media /media
ENV GOPHENBERG_WEB_DIR=/web
ENV GOPHENBERG_THEMES_DIR=/themes
ENV GOPHENBERG_MEDIA_DIR=/media
ENV GOPHENBERG_ADDR=0.0.0.0:8081
ENV GOPHENBERG_NODE_BIN=/nodejs/bin/node
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/gophenberg"]
