package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var version string
var buildTime string

func main() {
	var bucket, region, path, cannedACL, endpoint, jsonGrants string
	var dryrun, versionFlag bool
	var wg sync.WaitGroup
	var awsConfig = aws.Config{}
	var grants []*s3.Grant
	var success, counter int64

	flag.StringVar(&endpoint, "endpoint", "", "Endpoint URL")
	flag.StringVar(&region, "region", "", "AWS region")
	flag.StringVar(&bucket, "bucket", "", "Bucket name")
	flag.StringVar(&path, "path", "/", "Path to recurse under")
	flag.StringVar(&cannedACL, "acl", "private", "Canned ACL to assign objects")
	flag.BoolVar(&versionFlag, "version", false, "Display version and exit")
	flag.StringVar(&jsonGrants, "grants", "", "If set, acl flag is ignored. Grants part of ACL in json, ie : '[{\"Grantee\":{\"ID\":\"123456789\",\"Type\":\"CanonicalUser\"},\"Permission\":\"FULL_CONTROL\"}]'")
	flag.BoolVar(&dryrun, "dry-run", false, "Don't perform ACL operations, just list")

	flag.Parse()

	if versionFlag {
		fmt.Printf("s3-recursive-acl\nVersion : %s\nBuild time : %s\n", version, buildTime)
		os.Exit(0)
	}
	flagset := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagset[f.Name] = true })
	if !flagset["bucket"] {
		log.Fatal("bucket is mandatory!")
	}
	if !flagset["region"] {
		log.Fatal("region is mandatory!")
	}
	if !flagset["endpoint"] {
		awsConfig.Endpoint = aws.String(endpoint)
	}
	if flagset["grants"] {
		json.Unmarshal([]byte(jsonGrants), &grants)
	}
	awsConfig.Region = aws.String(region)

	svc := s3.New(session.New(), &awsConfig)

	err := svc.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Prefix: aws.String(path),
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, object := range page.Contents {
			key := *object.Key
			counter++
			wg.Add(1)
			go func(bucket string, key string, cannedACL string, grants []*s3.Grant) {
				defer wg.Done()
				var accessControlPolicy s3.AccessControlPolicy

				var aclParam *s3.PutObjectAclInput
				if grants != nil {
					out, err := svc.GetObjectAcl(&s3.GetObjectAclInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
					})
					if err != nil {
						log.Fatalf("Unable to read acl on %s, %v\n", key, err)
					}

					accessControlPolicy.Owner = out.Owner
					accessControlPolicy.Grants = grants
					aclParam = &s3.PutObjectAclInput{
						Bucket:              aws.String(bucket),
						Key:                 aws.String(key),
						AccessControlPolicy: &accessControlPolicy,
					}
				} else {
					aclParam = &s3.PutObjectAclInput{
						ACL:    aws.String(cannedACL),
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
					}
				}
				log.Printf("Updating '%s'\n", key)
				if !dryrun {
					_, err := svc.PutObjectAcl(aclParam)
					if err != nil {
						log.Printf("Failed to change permissions on '%s', %s\n", key, err)
					} else {
						success++
					}
				} else {
					success++
				}
			}(bucket, key, cannedACL, grants)
		}
		return true
	})

	wg.Wait()

	if err != nil {
		log.Panicf("Failed to update object permissions in '%s', %v\n", bucket, err)
	}
	if !dryrun {
		log.Printf("Updated permissions on %d objects out of %d\n", success, counter)
	} else {
		log.Printf("DRY RUN : Updated permissions on %d objects out of %d\n", success, counter)
	}
}
