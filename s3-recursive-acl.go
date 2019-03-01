package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	var bucket, region, path, cannedACL string
	var wg sync.WaitGroup
	var counter int64
	var changeError bool

	flag.StringVar(&region, "region", "ap-northeast-1", "AWS region")
	flag.StringVar(&bucket, "bucket", "s3-bucket", "Bucket name")
	flag.StringVar(&path, "path", "/", "Path to recurse under")
	flag.StringVar(&cannedACL, "acl", "public-read", "Canned ACL to assign objects")
	flag.Parse()

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		SharedConfigState:       session.SharedConfigEnable,
		Config:                  aws.Config{Region: aws.String(region), CredentialsChainVerboseErrors: aws.Bool(true)},
	}))

	svc := s3.New(sess, &aws.Config{
		Region: aws.String(region),
	})

	changeError = false

	listErr := svc.ListObjectsPages(&s3.ListObjectsInput{
		Prefix: aws.String(path),
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, object := range page.Contents {
			key := *object.Key
			counter++
			go func(bucket string, key string, cannedACL string) {
				wg.Add(1)
				_, err := svc.PutObjectAcl(&s3.PutObjectAclInput{
					ACL:    aws.String(cannedACL),
					Bucket: aws.String(bucket),
					Key:    aws.String(key),
				})
				fmt.Println(fmt.Sprintf("Updating '%s'", key))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to change permissions on '%s', %v", key, err)
					changeError = true
				}
				defer wg.Done()
			}(bucket, key, cannedACL)
		}
		return true
	})

	wg.Wait()

	if changeError {
		panic(fmt.Sprintf("Failed to update some object permissions in '%s'", bucket))
	}

	if listErr != nil {
		panic(fmt.Sprintf("Failed to list objects in '%s', %v", bucket, listErr))
	}

	fmt.Println(fmt.Sprintf("Successfully updated permissions on %d objects", counter))
}
