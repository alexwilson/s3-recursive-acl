package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/remeh/sizedwaitgroup"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	var bucket, region, path, cannedACL string
	var verbose bool
	var counter, s3concurrency int

	flag.StringVar(&region, "region", "us-east-1", "AWS region")
	flag.StringVar(&bucket, "bucket", "your-bucket", "Bucket name")
	flag.StringVar(&path, "path", "", "Path to recurse under (blank is root)")
	flag.StringVar(&cannedACL, "acl", "public-read", "Canned ACL to assign objects")
	flag.IntVar(&s3concurrency,"concurrency",50,"Number of requests per second to s3")
	flag.BoolVar(&verbose,"verbose",false,"Verbose Logging")
	flag.Parse()

	wg := sizedwaitgroup.New(s3concurrency)
	sess := session.Must(session.NewSession())

	svc := s3.New(sess, &aws.Config{
		Region: aws.String(region),
	})



	err := svc.ListObjectsPages(&s3.ListObjectsInput{
		Prefix: aws.String(path),
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, object := range page.Contents {
			key := *object.Key
			counter++

			go func(bucket string, key string, cannedACL string) {
				wg.Add()
				_, err := svc.PutObjectAcl(&s3.PutObjectAclInput{
					ACL:    aws.String(cannedACL),
					Bucket: aws.String(bucket),
					Key:    aws.String(key),
				})
				if (verbose == true){
					fmt.Println(fmt.Sprintf("Updating '%s'", key))
				}
				// singular retry for packet loss...
				if err != nil {
					if (verbose == true){
						fmt.Fprintf(os.Stderr, "Failed to change permissions on '%s', %v", key, err)
						fmt.Println(fmt.Sprintf("retrying %s",key))
					}
					_, err := svc.PutObjectAcl(&s3.PutObjectAclInput{
						ACL:    aws.String(cannedACL),
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
					})
					if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to retry permissions on '%s', %v", key, err)
					}
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
