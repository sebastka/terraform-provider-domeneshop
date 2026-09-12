BINARY  := terraform-provider-domeneshop
VERSION ?= dev

.PHONY: build test fmt vet lint golangci install clean docs

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

test:
	go test ./... -race

fmt:
	gofmt -w .

vet:
	go vet ./...

# golangci-lint is optional locally (dnf install golangci-lint) but runs in CI.
golangci:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; skipping (dnf install golangci-lint)"; \
	fi

lint: fmt vet golangci test

# Install into the local plugin directory used by dev_overrides. See README.
install: build
	mkdir -p ~/.local/share/terraform/plugins/registry.terraform.io/sebastka/domeneshop/$(VERSION)/$$(go env GOOS)_$$(go env GOARCH)
	cp $(BINARY) ~/.local/share/terraform/plugins/registry.terraform.io/sebastka/domeneshop/$(VERSION)/$$(go env GOOS)_$$(go env GOARCH)/

# Regenerate docs/ from the schemas and examples/ (needs tfplugindocs).
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name domeneshop

clean:
	rm -f $(BINARY)
