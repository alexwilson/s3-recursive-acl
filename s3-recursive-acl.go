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

	"github.com/posener/complete/v2/compflag"
	"github.com/posener/complete/v2/predict"
)

type dryrun struct {
	prefix string
	active bool
}

var version string
var buildTime string

var successCnt, matchedCnt, errorCnt, objectCnt int64
var dryRun *dryrun

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
				log.Printf("Failed to read acl on %s, %v\n", key, err)
				atomic.AddInt64(&errorCnt, 1)
				return
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

		log.Printf("%s Updating '%s'\n", dryRun.prefix, key)
		if !dryRun.active {
			// dry mode
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
	log.Printf("%s Summary : ACL changed : %d, objects matched regex : %d, total objects : %d, errors : %d\n", dryRun.prefix, successCnt, matchedCnt, objectCnt, errorCnt)
}

func main() {
	var bucket, profile, region, path, cannedACL, endpoint, includeRegex, jsonGrants string
	var dryrunFlag, versionFlag bool
	var parallel int
	var wg sync.WaitGroup
	var awsConfig = aws.Config{}
	var grants []*s3.Grant

	compflag.StringVar(&endpoint, "endpoint", "", "Endpoint URL", predict.OptValues(""))
	compflag.StringVar(&profile, "profile", "", "AWS credentials profile name", predict.OptValues(""))
	compflag.StringVar(&region, "region", "", "AWS region", predict.OptValues(""))
	compflag.IntVar(&parallel, "parallel", 32, "Number of parallel thread to run, a number too high may result in too many files open exception or hit a rate limit", predict.OptValues(""))
	compflag.StringVar(&bucket, "bucket", "", "Bucket name", predict.OptValues(""))
	compflag.StringVar(&path, "path", "", "Path to recurse under", predict.OptValues(""))
	compflag.BoolVar(&versionFlag, "version", false, "Display version and exit")
	compflag.StringVar(&includeRegex, "regex", ".*", "regex to include", predict.OptValues(""))
	compflag.StringVar(&jsonGrants, "grants", "", "If set, acl flag is ignored. Grants part of ACL in json, ie : '[{\"Grantee\":{\"ID\":\"123456789\",\"Type\":\"CanonicalUser\"},\"Permission\":\"FULL_CONTROL\"}]'", predict.OptValues(""))
	compflag.BoolVar(&dryrunFlag, "dry-run", false, "Don't perform ACL operations, just list", predict.OptValues("true", "false"))

	// https://docs.aws.amazon.com/AmazonS3/latest/userguide/acl-overview.html#canned-acl
	compflag.StringVar(&cannedACL, "acl", "private", "Canned ACL to assign objects", predict.OptValues(
		"private",
		"public-read",
		"public-read-write",
		"aws-exec-read",
		"authenticated-read",
		"bucket-owner-read",
		"bucket-owner-full-control",
	))

	compflag.Parse()

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
		if region, ok := os.LookupEnv("AWS_REGION"); ok == true {
			flag.Set("region", region)
		} else if region, ok := os.LookupEnv("AWS_DEFAULT_REGION"); ok == true {
			flag.Set("region", region)
		} else {
			log.Fatal("region is mandatory!")
		}
	}

	// Set `AWS_PROFILE` env var if flag was provided on command line
	if flagset["profile"] {
		os.Setenv("AWS_PROFILE", profile)
	}

	// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	// https://github.com/aws/aws-sdk-go#configuring-credentials
	// Enable SDK support for the shared configuration file if
	// respective variable is set in environment
	if _, ok := os.LookupEnv("AWS_SHARED_CREDENTIALS_FILE"); ok == true {
		if _, ok := os.LookupEnv("AWS_SDK_LOAD_CONFIG"); ok != true {
			os.Setenv("AWS_SDK_LOAD_CONFIG", "1")
		}
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
	dryRun = &dryrun{active: dryrunFlag}
	if dryrunFlag {
		dryRun.prefix = "DRY RUN:"
	}

	// Setup AWS SDK
	if !flagset["endpoint"] {
		awsConfig.Endpoint = aws.String(endpoint)
	}
	awsConfig.Region = aws.String(region)
	svc := s3.New(session.Must(session.NewSession()), &awsConfig)

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
				log.Printf("%s Skipping '%s'\n", dryRun.prefix, key)
			}
		}
		return true
	})

	if err != nil {
		log.Panicf("Failed to list objects  in '%s', %v\n", bucket, err)
	}

}
