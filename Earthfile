VERSION 0.7

# renovate: datasource=docker depName=golang
ARG --global GO_VERSION=1.20
# renovate: datasource=docker depName=golangci-lint
ARG --global GOLINT_VERSION=v1.51
# renovate: datasource=docker depName=quay.io/luet/base
ARG --global LUET_VERSION=0.34.0

luet:
    FROM quay.io/luet/base:$LUET_VERSION
    SAVE ARTIFACT /usr/bin/luet /luet

go-deps:
    ARG GO_VERSION
    FROM golang:$GO_VERSION-alpine
    WORKDIR /build
    COPY go.mod go.sum ./
    RUN go mod download
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

test:
    FROM +go-deps
    ENV CGO_ENABLED=0
    WORKDIR /build
    COPY +luet/luet /usr/bin/luet
    COPY . .
    RUN go run github.com/onsi/ginkgo/v2/ginkgo run --fail-fast --slow-spec-threshold 30s --covermode=atomic --coverprofile=coverage.out -p -r ./...
    SAVE ARTIFACT coverage.out AS LOCAL coverage.out

lint:
    FROM golangci/golangci-lint:$GOLINT_VERSION
    WORKDIR /build
    COPY . .
    RUN golangci-lint run