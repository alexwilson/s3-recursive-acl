# S3 Recursive ACL

An application for recursively setting canned ACL in an AWS S3 bucket.  Especially useful in large buckets.
# Usage
 `$ AWS_PROFILE=default ./s3-recursive-acl --bucket my-bucket-name-here --region region-here --path path/to/recurse --acl aws-exec-read --verbose true --concurrency 50`

# Build

Place in your $GOPATH folder and run `make build`. You may need to place the folder in a `src` parent directory.


| Canned ACL                | Applies to        | Permissions added to ACL                                                                                                                                  |
|---------------------------|-------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| private                   | Bucket and object | Owner gets FULL_CONTROL. No one else has access rights (default).                                                                                         |
| public-read               | Bucket and object | Owner gets FULL_CONTROL. The AllUsers group (see Who Is a Grantee?) gets READ access.                                                                     |
| public-read-write         | Bucket and object | Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access. Granting this on a bucket is generally not recommended.                           |
| aws-exec-read             | Bucket and object | Owner gets FULL_CONTROL. Amazon EC2 gets READ access to GET an Amazon Machine Image (AMI) bundle from Amazon S3.                                          |
| authenticated-read        | Bucket and object | Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.                                                                                   |
| bucket-owner-read         | Object            | Object owner gets FULL_CONTROL. Bucket owner gets READ access. If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.               |
| bucket-owner-full-control | Object            | Both the object owner and the bucket owner get FULL_CONTROL over the object. If you specify this canned ACL when creating a bucket, Amazon S3 ignores it. |
| log-delivery-write        | Bucket            | The LogDelivery group gets WRITE and READ_ACP permissions on the bucket.                                                                                  |
