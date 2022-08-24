VERSION=$(shell git describe --tags --always --long --dirty)
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")


all: clean build

clean:
	rm -f go.sum s3-recursive-acl 

install:
	go install ./s3-recursive-acl

build:
	go mod tidy
	go build -ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)" -o s3-recursive-acl s3-recursive-acl.go 
