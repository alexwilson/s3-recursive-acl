install:
	go install s3-recursive-acl

build:
	glide install
	go build s3-recursive-acl.go
