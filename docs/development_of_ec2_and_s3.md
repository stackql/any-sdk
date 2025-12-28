

## s3

S3 calls:


```bash

build/anysdk query \
  --svc-file-path="test/registry-simple/src/aws/v0.1.0/services/s3.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry-simple/src/aws/v0.1.0/provider.yaml" \
  --resource bucket_abac \
  --method get_bucket_abac \
  --parameters '{ "region": "ap-southeast-1", "Bucket": "stackql-trial-bucket-01" }' 

```

## ec2


ec2 calls:


```bash

build/anysdk query \
  --svc-file-path="test/registry-simple/src/aws/v0.1.0/services/ec2.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry-simple/src/aws/v0.1.0/provider.yaml" \
  --resource volumes_naively_presented \
  --method describeVolumes \
  --parameters '{ "region": "ap-southeast-2" }' 


build/anysdk query \
  --svc-file-path="test/registry-simple/src/aws/v0.1.0/services/ec2.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry-simple/src/aws/v0.1.0/provider.yaml" \
  --resource volumes_post_naively_presented \
  --method describeVolumes \
  --parameters '{ "region": "ap-southeast-2" }' 

```

Regression tests:

```bash

build/anysdk query \
    --svc-file-path="test/registry/src/aws/v0.1.0/services/ec2.yaml" \
    --tls.allowInsecure \
    --prov-file-path="test/registry/src/aws/v0.1.0/provider.yaml" \
    --resource volumes_presented \
    --method describeVolumes \
    --parameters '{ "region": "ap-southeast-2" }' 

```