gofmt: ## Runs gofmt against all packages. This is now subsumed by make golangci-lint.
	@echo GOFMT
	$(eval GOFMT_OUTPUT := $(shell gofmt -d -s server/ main.go 2>&1))
	@echo "$(GOFMT_OUTPUT)"
	@if [ ! "$(GOFMT_OUTPUT)" ]; then \
		echo "gofmt sucess"; \
	else \
		echo "gofmt failure"; \
		exit 1; \
	fi

## Old target to run go vet. Now it just invokes golangci-lint.
govet: golangci-lint
