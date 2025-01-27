
# CLI

The `any-sdk` CLI is for testing purposes, so long as semver < 1.


## Build

From the root of this repository:

```bash
cicd/cli/build_cli.sh
```

This creates an executable at the `.gitignore`d location `build/anysdk`.


## Examples

### Const

The `const` command is very much a trivial "Hello World":

```bash
./build/anysdk const
{"ExtensionKeyAlwaysRequired":"x-alwaysRequired"}
```

### Query


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
