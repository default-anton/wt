# All targets: show output only on failure (silent success)
# Unless V=1 is passed.

.PHONY: check test test-integration vet tidy goimports fmt vulncheck build

# Macro to run a command with optional verbosity
# Usage: $(call run,Label,Command)
define run
	@echo "==> $(1)"
	@if [ "$(V)" = "1" ]; then \
		$(2); \
	else \
		if OUTPUT=$$($(2) 2>&1); then \
			echo "✓ $(1) OK"; \
		else \
			echo "✗ $(1) FAILED"; \
			echo "$$OUTPUT"; \
			exit 1; \
		fi; \
	fi
endef

check: tidy goimports vet test test-integration vulncheck build

test:
	$(call run,go test ./... (unit),pkgs=$$(go list ./... | rg -v '/integration$$'); go test $$pkgs)

test-integration:
	$(call run,go test ./integration,go test ./integration)

vet:
	$(call run,go vet ./...,go vet ./...)

tidy:
	$(call run,go mod tidy,go mod tidy)

goimports:
	$(call run,goimports check,files=$$(goimports -l $$(go list -f '{{.Dir}}' ./...)); if [ -n "$$files" ]; then echo "$$files"; exit 1; fi)

fmt:
	$(call run,goimports -w,goimports -w $$(go list -f '{{.Dir}}' ./...))

vulncheck:
	$(call run,govulncheck ./...,govulncheck ./...)

build:
	$(call run,go build ./cmd/wt,go build ./cmd/wt)
