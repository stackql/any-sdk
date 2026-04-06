
# Automock testing

The `anysdk` CLI is for testing purposes, so long as semver < 1.


## Setup

You will need to install:

-  `stackql`, eg with `brew install stackql`.  There are other alternatives on [the official install doco](https://stackql.io/docs/installing-stackql). 
- `golang` >= `1.25.3`.
- `python3` >= `3.12`.


Once dependencies are in place, let us install the `anysdk` CLI.  From the root of this repository:

```bash
cicd/cli/build_cli.sh
```

Or if you want, cut out the middle man with `go build -o build/anysdk ./cmd/interrogate`.

This creates an executable at the `.gitignore`d location `build/anysdk`.

Set up the mock testing venv from the root of this repository:

```bash
python3 -m venv mock.venv

source mock.venv/bin/activate

pip install -r cicd/mock-testing-requirements.txt

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
  --mock-query-dir "cicd/out/mock-queries/aws" \
  --schema-dir \
  cicd/schema-definitions \
  --stdout-file "cicd/out/aot/${_now}-summary.json" \
  --stderr-file "cicd/out/aot/${_now}-analysis.jsonl"

```

Pick a method, eg: `aws` `ec2` instances describe and generate the closure:

```bash

build/anysdk closure \
  ./.stackql \
  ./.stackql/src/aws/v26.02.00377/provider.yaml \
  ec2 \
  --provider aws \
  --resource instances \
  --rewrite-url http://localhost:5050 \
  > cicd/out/closures/closure_ec2_instances.yaml

```

## Example run

Let us perform an example run with reference material: 


```bash

source mock.venv/bin/activate

# Start mock in background
python3 test/auto-mocks/reference/mock_aws_ec2_instances_describe.py --port 5050 &
MOCK_PID=$!
sleep 1

# Smoke test
curl -s -X POST http://localhost:5050/ -d "Action=DescribeInstances"

# Run StackQL against the closure registry
response=$(AWS_SECRET_ACCESS_KEY=fake AWS_ACCESS_KEY_ID=fake stackql \
  --http.log.enabled \
  --tls.allowInsecure \
  --registry "{ \"url\": \"file://$(pwd)/test/auto-mocks/reference/registry\", \"localDocRoot\": \"$(pwd)/test/auto-mocks/reference/registry\", \"verifyConfig\": { \"nopVerify\": true } }" \
  exec "$(cat test/auto-mocks/reference/query_aws_ec2_instances_describe.txt);" -o json)

echo "response: $response"

if [ "$response" != "$(cat test/auto-mocks/reference/expect_aws_ec2_instances_describe.txt)" ]; then
  echo "failed"
else 
  echo "success"
fi

# Cleanup
kill $MOCK_PID 2>/dev/null

```


## Generalizing

Let us perform a fully automated run: 


```bash

source mock.venv/bin/activate

provider="aws"
providerHandle="aws"
providerVersion="v26.02.00377"
service="ec2"
resource="volumes"

stackql exec "registry pull ${provider} ${providerVersion};"

# Generate closure with provider doc
build/anysdk closure \
  ./.stackql \
  ./.stackql/src/${providerHandle}/${providerVersion}/provider.yaml \
  "${service}" \
  --provider "${provider}" \
  --resource "${resource}" \
  --rewrite-url http://localhost:5050 \
  --output-dir "cicd/out/closures/${provider}"

closure_dir="cicd/out/closures/${provider}/${provider}_${service}_${resource}"
mock_file="cicd/out/auto-mocks/${provider}/mock_${provider}_${service}_${resource}_describe.py"
query="$(cat cicd/out/mock-queries/${provider}/query_${provider}_${service}_${resource}_describe.txt)"

# Start mock
python3 "$mock_file" --port 5050 &
MOCK_PID=$!
sleep 1

# Smoke test
curl -s -X POST http://localhost:5050/ -d "Action=DescribeVolumes"

# Run StackQL
response=$(AWS_SECRET_ACCESS_KEY=fake AWS_ACCESS_KEY_ID=fake stackql \
  --http.log.enabled \
  --tls.allowInsecure \
  --registry "{ \"url\": \"file://$(pwd)/${closure_dir}\", \"localDocRoot\": \"$(pwd)/${closure_dir}\", \"verifyConfig\": { \"nopVerify\": true } }" \
  exec "${query};" -o json)

echo "response: $response"

# Cleanup
kill $MOCK_PID 2>/dev/null

```
