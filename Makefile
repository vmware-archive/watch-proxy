PREFIX?=vmware-tanzu/quartermaster
TAG?=0.19

deps:
	go mod verify
	go mod tidy
	go mod vendor

test:
	go test ./config ./emitter ./metrics -v

image:
	docker build --pull -t $(PREFIX):$(TAG) .

push: image
	docker push $(PREFIX):$(TAG)

release: push

