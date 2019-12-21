all: clean build

clean:
	rm -f go.sum s3-recursive-acl 

install:
	go install ./s3-recursive-acl

build:
	go mod init main || :
	go build -o s3-recursive-acl s3-recursive-acl.go 
