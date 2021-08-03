GOPATH = $(shell go env GOPATH)
export GOFLAGS = -mod=mod

all: build

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o ./app/jetstream-test
	ls -al ./app/jetstream-test

# for a local push from your laptop
docker:
	@rm -f ./jetstream-test
	docker build --progress=plain -t jetstream-test:latest -f ./examples/pkg/Dockerfile .
	#docker push harbor.infra.local/phishlabs/jetstream-test:latest
	docker run -d --rm --name jetstream-test -v $(PWD):/dist jetstream-test:latest
	@echo "./jetstream-test on the docker host is an alpine arch binary"

set:
	set