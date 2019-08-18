package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	var bucket, region, path, cannedACL string
	var wg sync.WaitGroup
	var counter int64
	flag.StringVar(&region, "region", "ap-northeast-1", "AWS region")
	flag.StringVar(&bucket, "bucket", "s3-bucket", "Bucket name")
	flag.StringVar(&path, "path", "/", "Path to recurse under")
	flag.StringVar(&cannedACL, "acl", "public-read", "Canned ACL to assign objects")
	flag.Parse()

	svc := s3.New(session.New(), &aws.Config{
		Region: aws.String(region),
	})

	err := svc.ListObjectsPages(&s3.ListObjectsInput{
		Prefix: aws.String(path),
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, object := range page.Contents {
			key := *object.Key
			counter++
			wg.Add(1)
			go func(bucket string, key string, cannedACL string) {
				_, err := svc.PutObjectAcl(&s3.PutObjectAclInput{
					ACL:    aws.String(cannedACL),
					Bucket: aws.String(bucket),
					Key:    aws.String(key),
				})
				fmt.Println(fmt.Sprintf("Updating '%s'", key))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to change permissions on '%s', %v", key, err)
				}
				defer wg.Done()
			}(bucket, key, cannedACL)
		}
		return true
	})

	wg.Wait()

	if err != nil {
		panic(fmt.Sprintf("Failed to update object permissions in '%s', %v", bucket, err))
	}

	fmt.Println(fmt.Sprintf("Successfully updated permissions on %d objects", counter))
}
