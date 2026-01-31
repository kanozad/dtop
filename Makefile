.PHONY: fmt vet test lint tidy

PKGS=./...

fmt:
	go fmt $(PKGS)

vet:
	go vet $(PKGS)

test:
	go test $(PKGS)

lint:
	golangci-lint run

tidy:
	go mod tidy
