FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 \
	go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
	-o /out/adaptive-limiterd ./cmd/adaptive-limiterd

FROM scratch AS artifact
COPY --from=build /out/adaptive-limiterd /
