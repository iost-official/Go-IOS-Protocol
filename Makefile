GO = go

VERSION = 1.0.0
COMMIT = $(shell git rev-parse --short HEAD)
PROJECT = github.com/iost-official/go-iost
DOCKER_IMAGE = iostio/iost-node:$(VERSION)-$(COMMIT)
DOCKER_DEVIMAGE = iostio/iost-dev:$(VERSION)
TARGET_DIR = target

ifeq ($(shell uname),Darwin)
	export CGO_LDFLAGS=-L$(shell pwd)/vm/v8vm/v8/libv8/_darwin_amd64
	export CGO_CFLAGS=-I$(shell pwd)/vm/v8vm/v8/include/_darwin_amd64
	export DYLD_LIBRARY_PATH=$(shell pwd)/vm/v8vm/v8/libv8/_darwin_amd64
endif

ifeq ($(shell uname),Linux)
	export CGO_LDFLAGS=-L$(shell pwd)/vm/v8vm/v8/libv8/_linux_amd64
	export CGO_CFLAGS=-I$(shell pwd)/vm/v8vm/v8/include/_linux_amd64
	export LD_LIBRARY_PATH=$(shell pwd)/vm/v8vm/v8/libv8/_linux_amd64
endif
BUILD_TIME := $(shell date +%Y%m%d_%H%M%S%z)
LD_FLAGS := -X github.com/iost-official/go-iost/core/global.BUILD_TIME=$(BUILD_TIME) -X github.com/iost-official/go-iost/core/global.GIT_HASH=$(shell git rev-parse HEAD)

.PHONY: all build iserver iwallet lint test image devimage swagger protobuf install clean debug clear_debug_file

all: build

build: iserver iwallet

iserver:
	$(GO) build -ldflags "$(LD_FLAGS)" -o $(TARGET_DIR)/iserver $(PROJECT)/cmd/iserver

iwallet:
	$(GO) build -o $(TARGET_DIR)/iwallet $(PROJECT)/cmd/iwallet

lint:
	@gometalinter --config=.gometalinter.json ./...

test:
ifeq ($(origin VERBOSE),undefined)
	go test -coverprofile=coverage.txt -covermode=atomic ./...
else
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
endif

image:
	docker run --rm -v `pwd`:/gopath/src/github.com/iost-official/go-iost $(DOCKER_DEVIMAGE) make BUILD_TIME=$(BUILD_TIME)
	docker build -f Dockerfile.run -t $(DOCKER_IMAGE) .

devimage:
	docker build -f Dockerfile.dev -t $(DOCKER_DEVIMAGE) .

swagger:
	./script/gen_swagger.sh

protobuf:
	./script/gen_protobuf.sh

install:
	go install ./cmd/iwallet/
	go install ./cmd/iserver/

clean:
	rm -f ${TARGET_DIR}

debug: build
	target/iserver -f config/iserver.yml

clear_debug_file:
	rm -rf StatePoolDB/
	rm -rf leveldb/
	rm priv.key
	rm routing.table
