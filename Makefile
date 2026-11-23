GO ?= $(HOME)/sdk/go1.27.3/bin/go

.PHONY: build run test vet fmt tidy lint-sh test-sh
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
lint-sh:
	shellcheck install.sh install_test.sh
test-sh:
	bash install_test.sh
