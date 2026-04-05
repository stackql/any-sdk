#!/usr/bin/env bash

provider="aws"

providerHandle="aws" # differs for google and future proof aliasing

providerVersion="v26.02.00377"

service="ec2"

resource="volumes"

method="describe"

## Hoisted out of loop section

# stackql exec "registry pull ${provider} ${providerVersion};" 
# _now="$(date +%s)" && build/anysdk aot \
#   ./.stackql \
#   ./.stackql/src/aws/v26.02.00377/provider.yaml \
#   -v \
#   --mock-output-dir "cicd/out/auto-mocks/aws" \
#   --mock-expectation-dir "cicd/out/mock-expectations/aws" \
#   --mock-query-dir "cicd/out/mock-queries/aws" \
#   --schema-dir \
#   cicd/schema-definitions \
#   --stdout-file "cicd/out/aot/${_now}-summary.json" \
#   --stderr-file "cicd/out/aot/${_now}-analysis.jsonl"

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


container_id="$(docker run -d -p 5000:5000 -v ./cicd/out/auto-mocks:/opt/auto-mocks stackql/any-sdk-testlib:latest python /opt/auto-mocks/${provider}mock__${service}_${resource}_${method}.py --port 5000)"


docker exec $container_id curl -s -X POST http://localhost:5000/ -d "Action=DescribeInstances"



response=$(AWS_SECRET_ACCESS_KEY=fake AWS_ACCESS_KEY_ID=fake stackql --http.log.enabled --tls.allowInsecure --registry "{ \"url\": \"file://$(pwd)/test/auto-mocks/reference/registry\", \"localDocRoot\": \"$(pwd)/test/auto-mocks/reference/registry\", \"verifyConfig\": { \"nopVerify\": true } }" exec "$(cat test/auto-mocks/reference/query_aws_ec2_instances_describe.txt);" -o json)


echo "response is: $response"

if [ "$response" != "$(cat test/auto-mocks/reference/expect_aws_ec2_instances_describe.txt)" ]; then
  echo "failed"
else 
  echo "success"
fi

docker kill $container_id