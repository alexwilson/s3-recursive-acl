package main

import (
	"flag"
	"log"
	"net/http"
	"sync"

	"github.com/hashicorp/go-cleanhttp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	var bucket, region, path, cannedACL string
	var wg sync.WaitGroup
	var counter int64
	var dryRun bool
	var maxConn int
	flag.StringVar(&region, "region", "ap-northeast-1", "AWS region")
	flag.StringVar(&bucket, "bucket", "s3-bucket", "Bucket name")
	flag.StringVar(&path, "path", "/", "Path to recurse under")
	flag.StringVar(&cannedACL, "acl", "public-read", "Canned ACL to assign objects")
	flag.BoolVar(&dryRun, "dryrun", true, "do not change ACL")
	flag.IntVar(&maxConn, "maxconn", 100, "max. number of connections per host")
	flag.Parse()

	tr := cleanhttp.DefaultPooledTransport()
	tr.MaxConnsPerHost = maxConn
	svc := s3.New(session.Must(session.NewSession()), &aws.Config{
		Region: aws.String(region),
		HTTPClient: &http.Client{
			Transport: tr,
		},
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
				if dryRun {
					log.Printf("[DRYRUN] Updating '%s'", key)
					_, _ = svc.GetObjectAcl(&s3.GetObjectAclInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
					})
				} else {
					log.Printf("Updating '%s'", key)
					_, err := svc.PutObjectAcl(&s3.PutObjectAclInput{
						ACL:    aws.String(cannedACL),
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
					})
					if err != nil {
						log.Printf("Failed to change permissions on %q, %v", key, err)
					}
				}
				defer wg.Done()
			}(bucket, key, cannedACL)
		}
		return true
	})

	wg.Wait()

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Successfully updated permissions on %d objects", counter)
}
