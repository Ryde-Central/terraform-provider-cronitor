VERSION ?= dev
BINARY  := terraform-provider-cronitor
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)
# Local install location for terraform/tofu filesystem mirrors.
INSTALL_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/ryde/cronitor/$(VERSION:v%=%)/$(OS_ARCH)

.PHONY: build install test testacc lint docs fmt

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/

# Unit tests: run terraform/tofu against an in-memory fake of the Cronitor API.
test:
	go test ./... -count=1

# Acceptance tests: create, update, import and destroy real objects in the
# Cronitor account behind CRONITOR_API_KEY. Everything they create is prefixed tfacc-.
testacc:
	TF_ACC=1 go test ./internal/provider -run '^TestAcc' -count=1 -v -timeout 30m

lint:
	go vet ./...
	gofmt -l . | (! grep .)

fmt:
	gofmt -w .
	tofu fmt -recursive examples || terraform fmt -recursive examples

# Regenerate docs/ from the provider schema and examples/.
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name cronitor
