# S3 Recursive ACL

An application for recursively setting canned ACL or modify ACL Grants in an AWS S3 bucket. Especially useful in large buckets.

## Canned ACL

Example Usage: 

```bash
$ AWS_PROFILE=default ./s3-recursive-acl --bucket my-bucket-name-here --region region-here --path path/to/recurse --acl aws-exec-read
```

By default the `private` canned ACL is applied.

See https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl for latest available canned ACL.

## Dry run

You can add `--dry-run` to perform only `list-object-acl` and `get-object-acl` operations without performing `put-object-acl`.

## Specific Endpoint

You can use `--endpoint` to specify a different endpoint for connecting to S3.

## Specific Grants

The application can be used to update specific grants in the access control policy. The **original owner of the object is not changed.**

The flag is `grants` and the format is the same as the `Grants: []` structure returned by the `s3api get-object-acl` call. 

Example usage: 

```bash
$ AWS_PROFILE=default ./s3-recursive-acl --bucket my-bucket-name-here --region region-here --path path/to/recurse --grants '[{"Grantee":{"ID":"123456789","Type":"CanonicalUser"},"Permission":"FULL_CONTROL"}]'
```

Tip to get the proper structure right (using `jq` ) :

```bash
$ AWS_PROFILE=default aws s3api  get-object-acl --bucket my-bucket-name-here --key my-key-here | jq -c .Grants
```



