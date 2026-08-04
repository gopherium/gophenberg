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
FROM golang:1.26 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gophenberg ./cmd/gophenberg

# Assemble the runtime image: the binary plus the built SPA.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /gophenberg /gophenberg
COPY --from=frontend /app/frontend/dist /web
ENV GOPHENBERG_WEB_DIR=/web
ENV GOPHENBERG_ADDR=0.0.0.0:8081
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/gophenberg"]
