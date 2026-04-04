
# Automock testing

The `anysdk` CLI is for testing purposes, so long as semver < 1.


## Setup

You will need to install:

-  `stackql`, eg with `brew install stackql`.  There are other alternatives on [the official install doco](https://stackql.io/docs/installing-stackql). 
- `golang` >= `1.25.3`.
- `docker`.


Once dependencies are in place, let us install the `anysdk` CLI.  From the root of this repository:

```bash
cicd/cli/build_cli.sh
```

Or if you want, cut out the middle man with `go build -o build/anysdk ./cmd/interrogate`.

This creates an executable at the `.gitignore`d location `build/anysdk`.

From the root of this repository:

```bash
docker build -f testlib.Dockerfile -t stackql/any-sdk-testlib:latest .

```

Then, generate some auto mocks, for example `aws`, again from repository root:

```bash

stackql exec "registry pull aws v26.02.00377;"

_now="$(date +%s)" && build/anysdk aot \
  ./.stackql \
  ./.stackql/src/aws/v26.02.00377/provider.yaml \
  -v \
  --mock-output-dir "cicd/out/auto-mocks/aws" \
  --mock-expectation-dir "cicd/out/mock-expectations/aws" \
  --schema-dir \
  cicd/schema-definitions > "cicd/out/aot/${_now}-summary.json" 2>"cicd/out/aot/${_now}-analysis.jsonl"

```

Pick a method, eg: `aws` `ec2` instances describe and generate the closure:

```bash

build/anysdk closure \
  ./.stackql \
  ./.stackql/src/aws/v26.02.00377/provider.yaml \
  ec2 \
  --provider aws \
  --resource instances \
  --rewrite-url http://localhost:1091 \
  > cicd/out/closures/closure_ec2_instances.yaml

```

## Esxample run

Let us perform an example run with reference material: 


```bash


container_id="$(docker run -d -p 5000:5000 -v ./test/auto-mocks/reference:/opt/auto-mocks stackql/any-sdk-testlib:latest python /opt/auto-mocks/mock_aws_ec2_instances_describe.py --port 5000)"


docker exec $container_id curl -s -X POST http://localhost:5000/ -d "Action=DescribeInstances"


docker kill $container_id


response=$(AWS_SECRET_ACCESS_KEY=fake AWS_ACCESS_KEY_ID=fake stackql --http.log.enabled --tls.allowInsecure --registry "{ \"url\": \"file://$(pwd)/test/auto-mocks/reference/registry\", \"localDocRoot\": \"$(pwd)/test/auto-mocks/reference/registry\", \"verifyConfig\": { \"nopVerify\": true } }" exec "select * from aws.ec2.instances where region = 'ap-southeast-2';" -o json)


if [ "$response" != "$(cat test/auto-mocks/reference/expect_aws_ec2_instances_describe.txt)" ]; then
  echo "failed"
else 
  echo "success"
fi


# cicd/out/mock-expectations/aws/expect_aws_ec2_instances_describe.txt

```

## Examples

### Const

The `const` command is very much a trivial "Hello World":

```bash
./build/anysdk const
```

This returns:

```
{"ExtensionKeyAlwaysRequired":"x-alwaysRequired"}
```

### Query

HTTP Provider:

```bash

export GOOGLE_CREDENTIALS="$(cat cicd/keys/google-ro-credentials.json)"


./build/anysdk query \
  --svc-file-path="test/tmp/googleapis.com/v24.11.00274/services/compute.yaml" \
  --prov-file-path="test/tmp/googleapis.com/v24.11.00274/provider.yaml" \
  --resource accelerator_types \
  --method aggregated_list \
  --parameters '{ "project": "stackql-demo" }' \
  | jq -r '.items["zones/us-east7-b"]'


./build/anysdk query \
  --svc-file-path="test/tmp/googleapis.com/v24.11.00274/services/storage.yaml" \
  --prov-file-path="test/tmp/googleapis.com/v24.11.00274/provider.yaml" \
  --resource buckets \
  --method list \
  --parameters '{ "project": "stackql-demo" }' \
  | jq -r '.items[].id'

```

Local templated provider mutation:

```bash

./build/anysdk query \
  --svc-file-path="test/registry/src/local_openssl/v0.1.0/services/keys.yaml" \
  --prov-file-path="test/registry/src/local_openssl/v0.1.0/provider.yaml" \
  --resource rsa \
  --method create_key_pair \
  --parameters '{ 
			"config_file":   "test/openssl/openssl.cnf",
			"key_out_file":  "test/tmp/key.pem",
			"cert_out_file": "test/tmp/cert.pem",
			"days":          90
		}'

```

Local templated provider selection:

```bash

./build/anysdk query \
  --svc-file-path="test/registry/src/local_openssl/v0.1.0/services/keys.yaml" \
  --prov-file-path="test/registry/src/local_openssl/v0.1.0/provider.yaml" \
  --resource x509 \
  --method describe_certificate \
  --parameters '{
			"cert_file": "test/tmp/cert.pem"
		}'

```

For xml response trasformation on HTTP services...

In this `aws.ec2` example, you will first need to export your aws credential
env vars `AWS_SECRET_ACCESS_KEY` and `AWS_ACCESS_KEY_ID` and then this gives a nice `json` response:

```bash

build/anysdk query \
  --svc-file-path="test/registry/src/aws/v0.1.0/services/ec2.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry/src/aws/v0.1.0/provider.yaml" \
  --resource volumes_presented \
  --method describeVolumes \
  --parameters '{ "region": "ap-southeast-2" }' 

```

This one incorporates the hack for request translation:

```bash

build/anysdk query \
  --svc-file-path="test/registry-simple/src/aws/v0.1.0/services/ec2.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry-simple/src/aws/v0.1.0/provider.yaml" \
  --resource volumes_presented \
  --method describeVolumes \
  --parameters '{ "region": "ap-southeast-2" }' 

```

EC2 cod dev:

```bash

build/anysdk query \
  --svc-file-path="test/registry-simple/src/aws/v0.1.0/services/ec2.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry-simple/src/aws/v0.1.0/provider.yaml" \
  --resource volumes_presented \
  --method describeVolumes \
  --parameters '{ "region": "ap-southeast-2" }' 

```

S3 one of the great challenges:


```bash

build/anysdk query \
  --svc-file-path="test/registry-simple/src/aws/v0.1.0/services/s3.yaml" \
  --tls.allowInsecure \
  --prov-file-path="test/registry-simple/src/aws/v0.1.0/provider.yaml" \
  --resource bucket_abac \
  --method get_bucket_abac \
  --parameters '{ "region": "ap-southeast-1", "Bucket": "stackql-trial-bucket-01" }' 

```

### AOT analysis


```bash

build/anysdk aot \
  ./test/registry \
  ./test/registry/src/aws/v0.1.0/provider.yaml \
  -v \
  --schema-dir \
  cicd/schema-definitions

```

Feel like analyzing a live provider?  Download with stackql then go for it.

```bash

stackql exec "registry pull aws v26.02.00377;"

_now="$(date +%s)" && build/anysdk aot \
  ./.stackql \
  ./.stackql/src/aws/v26.02.00377/provider.yaml \
  -v \
  --mock-output-dir "cicd/out/auto-mocks/aws" \
  --schema-dir \
  cicd/schema-definitions > "cicd/out/aot/${_now}-summary.json" 2>"cicd/out/aot/${_now}-analysis.jsonl"

```

## Fine grained analysis


```bash

build/anysdk aot \
  test/registry \
  test/registry/src/aws/v0.1.0/provider.yaml \
  ec2 \
  --provider aws \
  --resource volumes_post_naively_presented \
  --schema-dir \
  cicd/schema-definitions


build/anysdk aot \
  test/registry \
  test/registry/src/aws/v0.1.0/provider.yaml \
  ec2 \
  --provider aws \
  --resource volumes_post_naively_presented \
  --method describeVolumes \
  --schema-dir \
  cicd/schema-definitions

```


## Closure Generation


```bash

build/anysdk closure \
  test/registry \
  test/registry/src/aws/v0.1.0/provider.yaml \
  ec2 \
  --provider aws \
  --resource volumes_post_naively_presented \
  --rewrite-url http://localhost:1091 \
  > cicd/out/aot/closure_ec2_volumes.yaml



```


## Auto-generated Flask mocks

The AOT analysis produces structured findings that include `sample_response`, `mock_route`, and `stackql_query` attributes for each analyzed method. These can be composed into runnable Flask mock servers for end-to-end testing.

### What the analysis emits per method

Each finding with a response transform includes:

| Field | Description |
|---|---|
| `sample_response.pre_transform` | Raw API response body (XML/JSON) derived from the provider's OpenAPI schema |
| `sample_response.post_transform` | Response after the provider's transform is applied |
| `mock_route` | Python string — a Flask route handler returning the mock response |
| `stackql_query` | The StackQL SQL query that exercises this endpoint |

### Composing a mock server

1. **Extract** — pull `mock_route` and `sample_response.pre_transform` from the JSONL analysis output
2. **Compose** — concatenate Flask boilerplate + all route strings with the mock response bodies injected:

```python
from flask import Flask, request, Response

app = Flask(__name__)

# ... paste mock_route strings here, with sample_response.pre_transform as the body ...

if __name__ == '__main__':
    app.run(port=1091)
```

3. **Reroute** — use an existing registry rewrite or server override to point the provider's base URL at `localhost:<port>`
4. **Test** — execute the `stackql_query` values against the rewritten registry:

```bash
# start mock
python mock_aws_ec2.py &

# run StackQL query from the analysis output
stackql exec \
  --registry='{"url": "file://./test/registry"}' \
  "SELECT * FROM aws.ec2.volumes_post_naively_presented WHERE region = 'us-east-1';"
```

This validates the full round-trip: StackQL sends a real request → Flask returns the schema-derived mock → the provider's response transform processes it → StackQL presents the result.

## Leveraging CLI for mock testing

For each closure, we can attach and instantiate corresponding method mocks from the appropriate generated mock file (when the parameter to persist these is populated with an output location).  These mocks can be run in containers.  Then we can test against these containers, verify, and terminate at the conclusion.

How to do this?

First, do somethig like this:

```bash

_now="$(date +%s)" && build/anysdk aot \
  ./.stackql \
  ./.stackql/src/aws/v26.02.00377/provider.yaml \
  -v \
  --mock-output-dir "cicd/out/auto-mocks/aws" \
  --schema-dir \
  cicd/schema-definitions > "cicd/out/aot/${_now}-summary.json" 2>"cicd/out/aot/${_now}-analysis.jsonl"

```

Initial proposal is repository root level docker compose file:

- Run 
- Mounts python file or files in for example `cicd/out/auto-mocks/aws`.
- Run `stackql` against the complementary closure.
- Verify that the result is as expected.  Confusingly, the expectation is still in a `jsonl` record.



