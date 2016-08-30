#
# go-service-helper
#

NAME	:= go-service-helper
DESC	:= helper packages for golang services
PREFIX	?= usr/local
VERSION := $(shell git describe --tags --always --dirty)
GOVERSION := $(shell go version)
BUILDTIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILDDATE := $(shell date -u +"%B %d, %Y")
BUILDER	:= $(shell echo "`git config user.name` <`git config user.email`>")
BUILD_NUMBER_FILE=.buildnum
BUILD_NUM := $(shell if [ -f ${BUILD_NUMBER_FILE} ]; then cat ${BUILD_NUMBER_FILE}; else echo 0; fi)
PKG_RELEASE ?= 1
PROJECT_URL := "git@github.com:mchudgins/go-service-helper.git"
LDFLAGS	:= -X 'main.version=$(VERSION)' \
	-X 'main.buildTime=$(BUILDTIME)' \
	-X 'main.builder=$(BUILDER)' \
	-X 'main.goversion=$(GOVERSION)' \
	-X 'main.buildNum=$(BUILD_NUM)'

.PHONY: fmt test fulltest install $(BUILD_NUMBER_FILE)

all: fmt install

fmt:
	go fmt
#	godep go fix .

install: pkg/serveSwagger/bindata_assetfs.go
	go install ./pkg/...

pkg/serveSwagger/bindata_assetfs.go:
	go get github.com/elazarl/go-bindata-assetfs/...
	@echo "Errors about 'no buildable Go source files' are expected at this next step..."
	@-go get -d github.com/swagger-api/swagger-ui
	go-bindata-assetfs -pkg serveSwagger -prefix \
		$(GOPATH)/src/github.com/swagger-api/swagger-ui/dist \
		$(GOPATH)/src/github.com/swagger-api/swagger-ui/dist \
		&& cp bindata_assetfs.go pkg/serveSwagger && rm bindata_assetfs.go

clean:
	rm pkg/serveSwagger/bindata*.go
