.PHONY: fmt vet test verify build

fmt:
	@files="$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"; if [ -n "$$files" ]; then printf 'gofmt needed:\n%s\n' "$$files"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

verify: fmt vet test

build:
	go build -o ./jkins ./cmd/jkins
