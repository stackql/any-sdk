#!/usr/bin/env bash

set -e

provider="aws"
providerHandle="aws"
providerVersion="v26.02.00377"
service="ec2"
resource="volumes"
method="describe"
MOCK_PORT=5050

## Activate venv
if [ ! -d "mock.venv" ]; then
  echo "Creating mock.venv..."
  python3 -m venv mock.venv
  mock.venv/bin/pip install -r cicd/mock-testing-requirements.txt
fi
source mock.venv/bin/activate

## Generate artifacts (uncomment to regenerate)
# stackql exec "registry pull ${provider} ${providerVersion};"
# _now="$(date +%s)" && build/anysdk aot \
#   ./.stackql \
#   ./.stackql/src/${providerHandle}/${providerVersion}/provider.yaml \
#   -v \
#   --mock-output-dir "cicd/out/auto-mocks/${provider}" \
#   --mock-expectation-dir "cicd/out/mock-expectations/${provider}" \
#   --mock-query-dir "cicd/out/mock-queries/${provider}" \
#   --schema-dir cicd/schema-definitions \
#   --stdout-file "cicd/out/aot/${_now}-summary.json" \
#   --stderr-file "cicd/out/aot/${_now}-analysis.jsonl"

## Generate closure with provider doc
build/anysdk closure \
  ./.stackql \
  ./.stackql/src/${providerHandle}/${providerVersion}/provider.yaml \
  "${service}" \
  --provider "${provider}" \
  --resource "${resource}" \
  --rewrite-url "http://localhost:${MOCK_PORT}" \
  --output-dir "cicd/out/closures/${provider}"

## Resolve files
mock_file="cicd/out/auto-mocks/${provider}/mock_${provider}_${service}_${resource}_${method}.py"
query_file="cicd/out/mock-queries/${provider}/query_${provider}_${service}_${resource}_${method}.txt"
expect_file="cicd/out/mock-expectations/${provider}/expect_${provider}_${service}_${resource}_${method}.txt"
closure_dir="$(pwd)/cicd/out/closures/${provider}/${provider}_${service}_${resource}"

query="$(cat "$query_file")"
expectation=""
[ -f "$expect_file" ] && [ -s "$expect_file" ] && expectation="$(cat "$expect_file")"

echo "query: $query"
echo "expectation: ${expectation:-<none>}"
echo "closure dir: $closure_dir"
echo "mock file: $(pwd)/$mock_file"

## Kill any leftover mock on this port
lsof -ti ":${MOCK_PORT}" 2>/dev/null | xargs kill -9 2>/dev/null || true

## Start mock
python3 "$mock_file" --port ${MOCK_PORT} &
MOCK_PID=$!
sleep 1

## Smoke test
echo ""
echo "=== Smoke test ==="
curl -s -X POST "http://localhost:${MOCK_PORT}/" -d "Action=DescribeVolumes" | head -100
echo ""

## Run StackQL
echo ""
echo "=== StackQL query ==="
response=$(AWS_SECRET_ACCESS_KEY=fake AWS_ACCESS_KEY_ID=fake stackql \
  --http.log.enabled \
  --tls.allowInsecure \
  --registry "{ \"url\": \"file://${closure_dir}\", \"localDocRoot\": \"${closure_dir}\", \"verifyConfig\": { \"nopVerify\": true } }" \
  exec "${query};" -o json 2>/tmp/stackql_stderr.txt)

echo "response: $response"

## Check result
http_status="$(grep 'http response status code:' /tmp/stackql_stderr.txt | head -1 | sed 's/.*status code: //' | sed 's/,.*//')"
echo "http status: ${http_status:-none}"

if [ -n "$expectation" ]; then
  if [ "$response" = "$expectation" ]; then
    echo "RESULT: PASS (body match)"
  else
    echo "RESULT: FAIL (body mismatch)"
  fi
elif [ "$http_status" = "200" ]; then
  echo "RESULT: PASS (status 200)"
else
  echo "RESULT: FAIL"
  cat /tmp/stackql_stderr.txt | head -5
fi

## Cleanup
kill $MOCK_PID 2>/dev/null
wait $MOCK_PID 2>/dev/null
