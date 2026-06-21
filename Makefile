GO_IMAGE ?= golang:1.26-alpine
GO_RUN = docker run --rm -u "$$(id -u):$$(id -g)" \
	-e HOME=/tmp -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/go-mod \
	-v "$$(pwd):/src" -w /src $(GO_IMAGE)

.PHONY: test vet fmt fmt-check build-freebsd clean

test:
	$(GO_RUN) go test ./...

vet:
	$(GO_RUN) go vet ./...

fmt:
	$(GO_RUN) gofmt -w $$(find . -name '*.go' -type f)

fmt-check:
	@test -z "$$($(GO_RUN) gofmt -l $$(find . -name '*.go' -type f))"

build-freebsd:
	mkdir -p dist
	$(GO_RUN) sh -c 'CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$${VERSION:-dev}" -o dist/adaptive-limiterd ./cmd/adaptive-limiterd'

clean:
	rm -rf build dist
