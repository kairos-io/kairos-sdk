VERSION 0.7

# renovate: datasource=docker depName=golang
ARG --global GO_VERSION=1.24
# renovate: datasource=docker depName=golangci-lint
ARG --global GOLINT_VERSION=1.61.0
# renovate: datasource=docker depName=quay.io/luet/base
ARG --global LUET_VERSION=0.34.0

luet:
    FROM quay.io/luet/base:$LUET_VERSION
    SAVE ARTIFACT /usr/bin/luet /luet

test:
    FROM golang:$GO_VERSION-alpine
    WORKDIR /build
    COPY . .
    RUN go mod tidy
    RUN go mod download
    RUN go mod verify
    ENV CGO_ENABLED=0
    WORKDIR /build
    COPY +luet/luet /usr/bin/luet
    # Some tests need the docker client available
    WITH DOCKER
        RUN go run github.com/onsi/ginkgo/v2/ginkgo run --fail-fast --covermode=atomic --coverprofile=coverage.out -p -r ./...
    END
    SAVE ARTIFACT coverage.out AS LOCAL coverage.out