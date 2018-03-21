all: deps 
	go build 

deps: ## Install/Update depdendencies
	dep ensure -v

test: ## Run tests
	go test ./... -v

image: ## Build Docker image
	docker build .

help: ## This help info
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: all deps test image
