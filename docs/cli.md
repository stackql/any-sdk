
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
