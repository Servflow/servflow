# Build stage
FROM golang:1.24 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o servflow .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/servflow servflow

ENV SERVFLOW_PORT=8080
ENV SERVFLOW_LOGLEVEL=production
EXPOSE 8080
USER root
ENTRYPOINT ["./servflow"]
