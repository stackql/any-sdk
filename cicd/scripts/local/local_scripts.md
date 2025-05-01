

Here is a typical example, from the root oif this repository, assuming you have the core repository locally at `../stackql-devel`:

```bash

env STACKQL_CORE_DIR="$(realpath ../stackql-devel)" cicd/scripts/local/02-build-stackql-test-package.sh

env STACKQL_CORE_DIR="$(realpath ../stackql-devel)" cicd/scripts/local/03-install-stackql-test-package.sh

env STACKQL_CORE_DIR="$(realpath ../stackql-devel)" cicd/scripts/local/04-setup-testing-dependencies.sh

env GCP_SERVICE_ACCOUNT_KEY="$(cat test/assets/credentials/dummy/google/functional-test-dummy-sa-key.json)" cicd/scripts/local/11-run-mocked.sh

```