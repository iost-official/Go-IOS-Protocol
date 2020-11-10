GO = go
GO_BUILD = go build -mod vendor
GO_TEST := go test -mod vendor -race -coverprofile=coverage.txt -covermode=atomic

VERSION = 3.3.6
COMMIT = $(shell git rev-parse --short HEAD)
PROJECT = github.com/iost-official/go-iost
DOCKER_IMAGE = iostio/iost-node:$(VERSION)-$(COMMIT)
DOCKER_RELEASE_IMAGE = iostio/iost-node:$(VERSION)
DOCKER_DEVIMAGE = iostio/iost-dev:$(VERSION)
TARGET_DIR = target
CLUSTER = devnet

ifeq ($(shell uname),Darwin)
	export CGO_LDFLAGS=-L$(shell pwd)/vm/v8vm/v8/libv8/_darwin_amd64
	export CGO_CFLAGS=-I$(shell pwd)/vm/v8vm/v8/include/_darwin_amd64
	export DYLD_LIBRARY_PATH=$(shell pwd)/vm/v8vm/v8/libv8/_darwin_amd64
	GO_TEST := $(GO_TEST) -exec "env DYLD_LIBRARY_PATH=$(DYLD_LIBRARY_PATH)" 
endif

ifeq ($(shell uname),Linux)
	export CGO_LDFLAGS=-L$(shell pwd)/vm/v8vm/v8/libv8/_linux_amd64
	export CGO_CFLAGS=-I$(shell pwd)/vm/v8vm/v8/include/_linux_amd64
	export LD_LIBRARY_PATH=$(shell pwd)/vm/v8vm/v8/libv8/_linux_amd64
endif
BUILD_TIME := $(shell date +%Y%m%d_%H%M%S%z)
LD_FLAGS := -X github.com/iost-official/go-iost/core/global.BuildTime=$(BUILD_TIME) -X github.com/iost-official/go-iost/core/global.GitHash=$(shell git rev-parse HEAD) -X github.com/iost-official/go-iost/core/global.CodeVersion=$(VERSION)

.PHONY: all build iserver iwallet itest lint test e2e_test k8s_test image push devimage swagger protobuf install clean debug clear_debug_file

all: build

build: iserver iwallet itest

iserver:
	$(GO_BUILD) -ldflags "$(LD_FLAGS)" -o $(TARGET_DIR)/iserver $(PROJECT)/cmd/iserver

iwallet:
	$(GO_BUILD) -o $(TARGET_DIR)/iwallet $(PROJECT)/cmd/iwallet

itest:
	$(GO_BUILD) -o $(TARGET_DIR)/itest $(PROJECT)/cmd/itest

lint:
	@golangci-lint run

vmlib:
	(cd vm/v8vm/v8/; make clean js_bin vm install deploy; cd ../../..)

vmlib_linux:
	docker run --rm -v `pwd`:/gopath/src/github.com/iost-official/go-iost $(DOCKER_DEVIMAGE) bash -c 'cd vm/v8vm/v8/ && make clean js_bin vm install'

test:
ifeq ($(origin VERBOSE),undefined)
	$(GO_TEST) ./...
else
	$(GO_TEST) -v ./...
endif

e2e_test: image
	docker rm -f iserver || true
	docker run -d --name iserver $(DOCKER_IMAGE)
	sleep 20
	docker exec -it iserver ./itest run a_case
	docker exec -it iserver ./itest run t_case
	docker exec -it iserver ./itest run c_case

k8s_test: image push
	./build/delete_cluster.sh $(CLUSTER)
	./build/create_cluster.sh $(CLUSTER)
	sleep 180
	kubectl exec -it itest -n $(CLUSTER) -- ./itest run -c /etc/itest/itest.json a_case
	kubectl exec -it itest -n $(CLUSTER) -- ./itest run -c /etc/itest/itest.json t_case
	kubectl exec -it itest -n $(CLUSTER) -- ./itest run -c /etc/itest/itest.json c_case

image:
	docker run --rm -v `pwd`:/gopath/src/github.com/iost-official/go-iost $(DOCKER_DEVIMAGE) make BUILD_TIME=$(BUILD_TIME)
	docker build -f Dockerfile.run -t $(DOCKER_IMAGE) .

push:
	docker push $(DOCKER_IMAGE)

devimage:
	docker build -f Dockerfile.dev -t $(DOCKER_DEVIMAGE) .

devpush:
	docker push $(DOCKER_DEVIMAGE)

release: devimage devpush
	docker run --rm -v `pwd`:/gopath/src/github.com/iost-official/go-iost $(DOCKER_DEVIMAGE) make BUILD_TIME=$(BUILD_TIME)
	docker build -f Dockerfile -t $(DOCKER_RELEASE_IMAGE) .
	docker push $(DOCKER_RELEASE_IMAGE)

swagger:
	./script/gen_swagger.sh

protobuf:
	./script/gen_protobuf.sh

install:
	go install -ldflags "$(LD_FLAGS)" ./cmd/iserver/
	go install ./cmd/iwallet/
	go install ./cmd/itest/

clean:
	rm -rf ${TARGET_DIR}

debug: build
	target/iserver -f config/iserver.yml

clear_debug_file:
	rm -rf storage
	rm -f p2p/priv.key
	rm -f p2p/routing.table
