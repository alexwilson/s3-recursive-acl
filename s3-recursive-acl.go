package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type dryrun struct {
	prefix string
	active bool
}

var version string
var buildTime string

var successCnt, matchedCnt, errorCnt, objectCnt int64
var dr dryrun

/*

This function returns a function ready to be sent to a channel to be executed

The function returned changes the ACL of an object and it supports two modes of operation :

One with a cannedACL, which simply apply the cannedACL to the object located at bucket/key

One with an array of s3.Grant.
When the list of grant is passed to this function, it takes priority over cannedACL.
A GetObjectAcl is called to retreive the Owner and a new access control policy is created
and passed along with the owner of the object to a PutObjectAcl call on the object located at bucket/key

Counters are affected during the execution of the function returned.

*/
func changeACL(svc *s3.S3, bucket string, key string, cannedACL string, grants []*s3.Grant) func() {
	return func() {
		var accessControlPolicy s3.AccessControlPolicy

		var aclParam *s3.PutObjectAclInput
		if grants != nil {
			out, err := svc.GetObjectAcl(&s3.GetObjectAclInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				log.Fatalf("Failed to read acl on %s, %v\n", key, err)
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

		log.Printf("%s Updating '%s'\n", dr.prefix, key)
		if !dr.active {
			_, err := svc.PutObjectAcl(aclParam)
			if err != nil {
				log.Printf("Failed to change permissions on '%s', %s\n", key, err)
				atomic.AddInt64(&errorCnt, 1)
			} else {
				atomic.AddInt64(&successCnt, 1)
			}
		} else {
			atomic.AddInt64(&successCnt, 1)
		}
	}
}
func stat() {
	log.Printf("%s Summary : ACL changed : %d, objects matched regex : %d, total objects : %d, errors : %d\n", dr.prefix, successCnt, matchedCnt, objectCnt, errorCnt)
}

func main() {
	var bucket, region, path, cannedACL, endpoint, includeRegex, jsonGrants string
	var dryrunFlag, versionFlag bool
	var parallel int
	var wg sync.WaitGroup
	var awsConfig = aws.Config{}
	var grants []*s3.Grant

	flag.StringVar(&endpoint, "endpoint", "", "Endpoint URL")
	flag.StringVar(&region, "region", "", "AWS region")
	flag.IntVar(&parallel, "parallel", 512, "Number of parallel thread to run, a number too high may result in too many files open exception or hit a rate limit")
	flag.StringVar(&bucket, "bucket", "", "Bucket name")
	flag.StringVar(&path, "path", "", "Path to recurse under")
	flag.StringVar(&cannedACL, "acl", "private", "Canned ACL to assign objects")
	flag.BoolVar(&versionFlag, "version", false, "Display version and exit")
	flag.StringVar(&includeRegex, "regex", ".*", "regex to include")
	flag.StringVar(&jsonGrants, "grants", "", "If set, acl flag is ignored. Grants part of ACL in json, ie : '[{\"Grantee\":{\"ID\":\"123456789\",\"Type\":\"CanonicalUser\"},\"Permission\":\"FULL_CONTROL\"}]'")
	flag.BoolVar(&dryrunFlag, "dry-run", false, "Don't perform ACL operations, just list")

	flag.Parse()

	flagset := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagset[f.Name] = true })

	// Print version if requested
	if versionFlag {
		fmt.Printf("s3-recursive-acl\nVersion : %s\nBuild time : %s\n", version, buildTime)
		os.Exit(0)
	}
	// Check if parallel flag is valid
	if parallel < 1 {
		log.Fatal("parallel must be larger than 1")
	}

	// Check mandatory flags
	if !flagset["bucket"] {
		log.Fatal("bucket is mandatory!")
	}
	if !flagset["region"] {
		log.Fatal("region is mandatory!")
	}

	// Convert json to object for grants
	if flagset["grants"] {
		json.Unmarshal([]byte(jsonGrants), &grants)
	}

	// Setup regex
	regex, err := regexp.Compile(includeRegex)
	if err != nil {
		log.Panicf("Failed to compile regex '%s', %v\n", includeRegex, err)
	}

	// Setup dryrun
	dr := &dryrun{active: dryrunFlag}
	if dryrunFlag {
		dr.prefix = "DRY RUN:"
	}

	// Setup AWS SDK
	if !flagset["endpoint"] {
		awsConfig.Endpoint = aws.String(endpoint)
	}
	awsConfig.Region = aws.String(region)
	svc := s3.New(session.New(), &awsConfig)

	// Setup parallel jobs and channel, and graceful exit
	jobs := make(chan func())
	defer stat()
	defer wg.Wait()
	defer close(jobs)
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				j()

			}
		}()

	}

	// List objects
	err = svc.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Prefix: aws.String(path),
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, object := range page.Contents {
			key := *object.Key
			objectCnt++
			if regex.MatchString(key) {
				matchedCnt++
				// send change ACL function to channel
				jobs <- changeACL(svc, bucket, key, cannedACL, grants)

			} else {
				log.Printf("%s Skipping '%s'\n", dr.prefix, key)
			}
		}
		return true
	})

	if err != nil {
		log.Panicf("Failed to list objects  in '%s', %v\n", bucket, err)
	}

}
