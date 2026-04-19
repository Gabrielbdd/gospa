# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json ./
RUN npm install --no-audit --no-fund --ignore-scripts
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# web/dist is excluded via .dockerignore so the host's stale dist does not
# leak into the build context; pull the fresh Vite build from the frontend
# stage so //go:embed all:dist has real content at compile time.
COPY --from=frontend /src/web/dist ./web/dist

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/app /app

USER nonroot:nonroot
EXPOSE 3000
ENTRYPOINT ["/app"]
