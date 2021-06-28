GO = go
GO_BUILD = $(GO) build -mod vendor
GO_TEST := $(GO) test -mod vendor -race -timeout 600s
GO_INSTALL := $(GO) install -mod vendor

PROJECT_NAME := $(shell basename "$(PWD)")
BUILDER_VERSION = 3.5.0
VERSION = 3.6.0
COMMIT = $(shell git rev-parse --short HEAD)
PROJECT = github.com/iost-official/go-iost
DOCKER_IMAGE = iostio/iost-node:$(VERSION)-$(COMMIT)
DOCKER_RELEASE_IMAGE = iostio/iost-node:$(VERSION)
DOCKER_DEVIMAGE = iostio/iost-dev:$(BUILDER_VERSION)
TARGET_DIR = target
DEV_DOCKER_RUN = docker run --rm -v `pwd`:/go-iost $(DOCKER_DEVIMAGE)

export GOBASE = $(shell pwd)
export GOARCH=amd64
export CGO_ENABLED=1

ifeq ($(shell uname),Darwin)
	#export CGO_LDFLAGS=-L$(shell pwd)/vm/v8vm/v8/libv8/_darwin_amd64
	#export CGO_CFLAGS=-I$(shell pwd)/vm/v8vm/v8/include/_darwin_amd64
	export DYLD_LIBRARY_PATH=$(shell pwd)/vm/v8vm/v8/libv8/_darwin_amd64
	GO_TEST := $(GO_TEST) -exec "env DYLD_LIBRARY_PATH=$(DYLD_LIBRARY_PATH)" 
endif

ifeq ($(shell uname),Linux)
	export CGO_LDFLAGS=-L$(shell pwd)/vm/v8vm/v8/libv8/_linux_amd64
	export CGO_CFLAGS=-I$(shell pwd)/vm/v8vm/v8/include/_linux_amd64
	export LD_LIBRARY_PATH=$(shell pwd)/vm/v8vm/v8/libv8/_linux_amd64
endif

BUILD_TIME := $(shell date +%Y%m%d_%H%M%S%z)
LD_FLAGS := -X github.com/iost-official/go-iost/v3/core/global.BuildTime=$(BUILD_TIME) -X github.com/iost-official/go-iost/v3/core/global.GitHash=$(shell git rev-parse HEAD) -X github.com/iost-official/go-iost/v3/core/global.CodeVersion=$(VERSION)

.PHONY: all build iserver iwallet itest lint test e2e_test image push devimage swagger protobuf install clean debug clear_debug_file env

all: build

build: iserver iwallet itest

iserver: $(eval SHELL:=/bin/bash) 
	$(GO_BUILD) -ldflags "$(LD_FLAGS)" -o $(TARGET_DIR)/iserver ./cmd/iserver
	@if [[ "`uname`" == "Darwin"* ]]; then \
		echo change libvm dylib path; \
		install_name_tool -change libvm.dylib $(DYLD_LIBRARY_PATH)/libvm.dylib ./target/iserver; \
	fi

iwallet:
	$(GO_BUILD) -o $(TARGET_DIR)/iwallet ./cmd/iwallet

itest:
	$(GO_BUILD) -o $(TARGET_DIR)/itest ./cmd/itest

lint:
	golangci-lint run

vmlib:
	(cd vm/v8vm/v8/; make clean js_bin vm install; cd ../../..)

vmlib_install:
	(cd vm/v8vm/v8/; make deploy; cd ../../..)

vmlib_linux:
	$(DEV_DOCKER_RUN) bash -c 'cd vm/v8vm/v8/ && make clean js_bin vm install'

test:
	$(GO) clean -testcache
ifeq ($(origin VERBOSE),undefined)
	$(GO_TEST) ./...
else
	$(GO_TEST) -v ./...
endif

e2e_test_local:
	$(TARGET_DIR)/itest run a_case
	$(TARGET_DIR)/itest run t_case
	$(TARGET_DIR)/itest run c_case

e2e_test: image
	docker rm -f iserver || true
	docker run -d --name iserver $(DOCKER_IMAGE)
	sleep 20
	docker exec -it iserver ./itest run a_case
	docker exec -it iserver ./itest run t_case
	docker exec -it iserver ./itest run c_case

image:
	$(DEV_DOCKER_RUN) make BUILD_TIME=$(BUILD_TIME)
	docker build -f Dockerfile.run -t $(DOCKER_IMAGE) .

release_image:
	$(DEV_DOCKER_RUN) make BUILD_TIME=$(BUILD_TIME)
	docker build -f Dockerfile -t $(DOCKER_RELEASE_IMAGE) .

push:
	docker push $(DOCKER_IMAGE)

devimage:
	docker build -f Dockerfile.dev -t $(DOCKER_DEVIMAGE) .

docker_build:
	$(DEV_DOCKER_RUN) make build

docker_lint:
	$(DEV_DOCKER_RUN) make lint

docker_test:
	$(DEV_DOCKER_RUN) make test

dev_image_tag:
	docker tag $(DOCKER_DEVIMAGE) iostio/iost-dev:latest

docker_full_test: devimage dev_image_tag image docker_lint docker_test e2e_test

devpush: dev_image_tag
	docker push $(DOCKER_DEVIMAGE)
	docker push iostio/iost-dev:latest

release: devimage devpush release_image
	docker push $(DOCKER_RELEASE_IMAGE)

protobuf:
	./script/gen_protobuf.sh

install:
	$(GO_INSTALL) -ldflags "$(LD_FLAGS)" ./cmd/iserver/
	$(GO_INSTALL) ./cmd/iwallet/
	$(GO_INSTALL) ./cmd/itest/

clean:
	rm -rf ${TARGET_DIR}

debug: build
	target/iserver -f config/iserver.yml

clear_debug_file:
	rm -rf storage logs ilog/logs1 ilog/logs2
	rm -f p2p/priv.key
	rm -f p2p/routing.table

env:
	@echo export GOBASE=$(GOBASE)
	@echo export GOARCH=$(GOARCH)
	@echo export CGO_ENABLED=$(CGO_ENABLED)
	@echo export CGO_CFLAGS=$(CGO_CFLAGS)
	@echo export CGO_LDFLAGS=$(CGO_LDFLAGS)
	@echo export LD_LIBRARY_PATH=$(LD_LIBRARY_PATH)
	@echo export DYLD_LIBRARY_PATH=$(DYLD_LIBRARY_PATH)
