.PHONY:	all build push

PREFIX=quay.io/reverbnation
PROJECT=$(shell basename $(CURDIR))
TAG=$(shell git rev-parse --short HEAD)
ROOT=$(shell git rev-parse --show-toplevel)
PACKAGE_DIR=$(ROOT)/package
SRC_VOLUME="$(ROOT)":/go/src/github.com/davars/sohop
IMAGE=golang:1.12-alpine3.10

all: build push

build:
	docker run --rm -v $(SRC_VOLUME) -v "$(PACKAGE_DIR)":/go/bin $(IMAGE) sh -c 'CGO_ENABLED=0 go get -v github.com/davars/sohop/cmd/sohop'
	docker build --pull -t $(PREFIX)/$(PROJECT):$(TAG) "$(PACKAGE_DIR)"
	rm "$(PACKAGE_DIR)/sohop"

push:
	docker push $(PREFIX)/$(PROJECT):$(TAG)
