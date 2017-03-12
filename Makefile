#
# go-service-helper
#

NAME	:= go-service-helper
DESC	:= helper packages for golang services
PROJECT_URL := "git@github.com:mchudgins/go-service-helper.git"

.PHONY: fmt test fulltest install $(BUILD_NUMBER_FILE)

all: fmt install

fmt:
	go fmt ./...

install: serveSwagger/assets.go
	go install ./...

serveSwagger/assets.go:
	@echo "Errors about 'no buildable Go source files' are expected at this next step..."
	@-go get -d github.com/swagger-api/swagger-ui
	staticfiles -o serveSwagger/assets.go ../../swagger-api/swagger-ui/dist

clean:
	rm serveSwagger/assets.go
