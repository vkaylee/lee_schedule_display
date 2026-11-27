FROM oven/bun:1.4.1 AS bun
FROM golang:1.26.11-bookworm
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN go install github.com/air-verse/air@v1.61.7
WORKDIR /workspace
