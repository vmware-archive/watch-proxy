PREFIX?=heptio/quartermaster
TAG?=k8s-1.7_qm-0.12

all: deps 
	go build 

deps: ## Install/Update depdendencies
	dep ensure -v

test: ## Run tests
	go test ./... -v

image: ## Build Docker image
	docker build .

run: ## Build Docker image
	go run main.go

help: ## This help info
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

server: main.go 
	CGO_ENABLED=0 GOOS=linux GOARCH= GOARM=6 go build -o server main.go

container: server
	docker build --pull -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

.PHONY: all deps test image
