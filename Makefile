install:
	go install ./s3-recursive-acl

build:
	dep ensure
	go build s3-recursive-acl.go
