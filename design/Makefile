#! /usr/bin/make
#
# Makefile for Goa examples
#
# Targets:
# - "depend" retrieves the Go packages needed to run the linter and tests
# - "gen" invokes the "goa" tool to generate the examples source code
# - "build" compiles the example microservices and client CLIs
# - "clean" deletes the output of "build"
# - "lint" runs the linter and checks the code format using goimports
# - "test" runs the tests
#
# Meta targets:
# - "all" is the default target, it runs all the targets in the order above.
#
GO_FILES=$(shell find . -type f -name '*.go')
GOA:=$(shell goa version 2> /dev/null)
GOOS=$(shell go env GOOS)
GOPATH=$(shell go env GOPATH)
GIT_ROOT=$(shell git rev-parse --show-toplevel)

export GO111MODULE=on

# Only list test and build dependencies
# Standard dependencies are installed via go get
DEPEND=\
	google.golang.org/protobuf/cmd/protoc-gen-go@latest \
	google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest \
	honnef.co/go/tools/cmd/staticcheck@latest \
	goa.design/goa/v3/cmd/goa@v3

.phony: all depend lint test build clean

all: check-goa gen lint test
	@echo DONE!

ci: depend all

# Install protoc
PROTOC_VERSION=21.7
UNZIP=unzip
ifeq ($(GOOS),linux)
	PROTOC=protoc-$(PROTOC_VERSION)-linux-x86_64
	PROTOC_EXEC=$(PROTOC)/bin/protoc
endif
ifeq ($(GOOS),darwin)
	PROTOC=protoc-$(PROTOC_VERSION)-osx-x86_64
	PROTOC_EXEC=$(PROTOC)/bin/protoc
endif
ifeq ($(GOOS),windows)
	PROTOC=protoc-$(PROTOC_VERSION)-win32
	PROTOC_EXEC="$(PROTOC)\bin\protoc.exe"
	GOPATH:=$(subst \,/,$(GOPATH))
	GIT_ROOT:=$(subst \,/,$(GIT_ROOT))
endif

# Note: the steps below rely on curl and tar which are available
# on both Linux and Windows 10 (build>=17603).
depend:
	@echo INSTALLING DEPENDENCIES...
	@go mod download
	@for package in $(DEPEND); do go install $$package; done
	@go mod tidy -compat=1.19
	@echo INSTALLING PROTOC...
	@mkdir $(PROTOC)
	@cd $(PROTOC); \
	curl -O -L https://github.com/google/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC).zip; \
	$(UNZIP) $(PROTOC).zip
	@cp $(PROTOC_EXEC) $(GOPATH)/bin && \
		rm -rf $(PROTOC) && \
		echo "`protoc --version`"
	@echo go mod graph

check-goa:
ifdef GOA
	go mod download
	@echo $(GOA)
else
	go get -u goa.design/goa/v3@v3
	go get -u goa.design/goa/v3/...@v3
	go mod download
	@echo $(GOA)
endif

lint:
	@echo LINTING CODE...
ifneq ($(GOOS),windows)
	@if [ "`staticcheck ./... | grep -v ".pb.go" | tee /dev/stderr`" ]; then \
		echo "^ - staticcheck errors!" && echo && exit 1; \
	fi
endif

gen:
	@# NOTE: not all command line tools are generated
	@echo GENERATING CODE...
	@goa version
	@cd $(GIT_ROOT)
	@echo "removing files generated Goa"
	@rm -rf "design"
	@echo "regenerating...."
	@goa gen github.com/niiniyare/awo/design 
	@goa example github.com/niiniyare/awo/design 
	@go mod tidy -compat=1.19

build:
	@cd "$(GIT_ROOT)/design" && \
		go build ./cmd/calc && go build ./cmd/calc-cli

test: ## 
	@echo TESTING...
	@go test ./design/... > /dev/null
