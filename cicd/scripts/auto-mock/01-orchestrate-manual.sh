#!/usr/bin/env bash

provider="aws"

providerHandle="aws" # differs for google and future proof aliasing

providerVersion="v26.02.00377"

service="ec2"

resource="volumes"

method="describe"

## Hoisted out of loop section

stackql exec "registry pull ${provider} ${providerVersion};" 
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

## End Hoisted out of loop section


mkdir -p "cicd/out/closures/${provider}_${service}_${resource}/src/${providerHandle}/${providerVersion}/services"

read -r -d '' PROVIDER_FILE_TMPL << EOF
id: ${provider}:${providerVersion}
name: ${provider}
version: ${providerVersion}
providerServices:
  ${service}:
    description: ${service}
    id: ${service}:${providerVersion}
    name: ${service}
    preferred: true
    service:
      \$ref: ${providerHandle}/${providerVersion}/services/${service}.yaml
    title: ${service} API
    version: ${providerVersion}
openapi: 3.0.0
config: # this should be copied from actual provider and paste here unchanged
  auth:
    type: "aws_signing_v4"
    credentialsenvvar: "AWS_SECRET_ACCESS_KEY"
    keyIDenvvar: "AWS_ACCESS_KEY_ID"
EOF

{
    echo "$PROVIDER_FILE_TMPL"
} > "./cicd/out/closures/${provider}_${service}_${resource}/src/${providerHandle}/${providerVersion}/provider.yaml"



build/anysdk closure \
  ./.stackql \
  ./.stackql/src/${providerHandle}/${providerVersion}/provider.yaml \
  "${service}" \
  --provider "${provider}" \
  --resource "${resource}" \
  --rewrite-url http://localhost:5000 \
  > "cicd/out/closures/${provider}_${service}_${resource}/src/${providerHandle}/${providerVersion}/services/${service}.yaml"

query="$(cat cicd/out/mock-queries/${provider}/query_${provider}_${service}_${resource}_${method}.txt)"

# eg: cicd/out/mock-expectations/aws/expect_aws_ec2_volumes_describe.txt
expectation="$(cat cicd/out/mock-expectations/${provider}/expect_${provider}_${service}_${resource}_${method}.txt)"

echo "query is: $query"
echo "expectation is: $expectation"


mock_file="mock_${provider}_${service}_${resource}_${method}.py"
registry_dir="$(pwd)/cicd/out/closures/${provider}_${service}_${resource}"

container_id="$(docker run -d -p 5000:5000 -v "$(pwd)/cicd/out/auto-mocks/${provider}:/opt/auto-mocks" stackql/any-sdk-testlib:latest python "/opt/auto-mocks/${mock_file}" --port 5000)"

# Wait for Flask to start
sleep 2

# Smoke test the mock
docker exec "$container_id" curl -s -X POST http://localhost:5000/ -d "Action=DescribeInstances" || echo "smoke test failed"

# Run StackQL against the closure registry
response=$(AWS_SECRET_ACCESS_KEY=fake AWS_ACCESS_KEY_ID=fake stackql \
  --http.log.enabled \
  --tls.allowInsecure \
  --registry "{ \"url\": \"file://${registry_dir}\", \"localDocRoot\": \"${registry_dir}\", \"verifyConfig\": { \"nopVerify\": true } }" \
  exec "${query};" -o json)

echo "response is: $response"

if [ "$response" != "$expectation" ]; then
  echo "failed"
else
  echo "success"
fi

docker kill "$container_id"