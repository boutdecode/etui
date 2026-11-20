GO ?= $(HOME)/sdk/go1.27.3/bin/go

.PHONY: build run test vet fmt tidy
build:
	$(GO) build -o bin/etui .
run:
	$(GO) run . $(ARGS)
test:
	$(GO) test ./...
vet:
	$(GO) vet ./...
fmt:
	$(GO)fmt -l -w .
tidy:
	$(GO) mod tidy
