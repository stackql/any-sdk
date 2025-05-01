#!/usr/bin/env bash

>&2 echo "requires all of requirements.txt"

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/context.sh"

# Check if the virtual environment exists
if [ ! -d "${STACKQL_CORE_DIR}/.venv" ]; then
    >&2 echo "Virtual environment not found. Please create it first."
    exit 1
fi

source "${STACKQL_CORE_DIR}/.venv/bin/activate"

python ${STACKQL_CORE_DIR}/test/python/stackql_test_tooling/registry_rewrite.py --srcdir "${REPOSITORY_ROOT_DIR}/test/registry/src" --destdir "${REPOSITORY_ROOT_DIR}/test/registry-mocked/src"

openssl req -x509 -keyout ${REPOSITORY_ROOT_DIR}/test/credentials/pg_server_key.pem  -out ${REPOSITORY_ROOT_DIR}/test/credentials/pg_server_cert.pem   -config ${STACKQL_CORE_DIR}/test/server/mtls/openssl.cnf -days 365
openssl req -x509 -keyout ${REPOSITORY_ROOT_DIR}/test/credentials/pg_client_key.pem  -out ${REPOSITORY_ROOT_DIR}/test/credentials/pg_client_cert.pem   -config ${STACKQL_CORE_DIR}/test/server/mtls/openssl.cnf -days 365
openssl req -x509 -keyout ${REPOSITORY_ROOT_DIR}/test/credentials/pg_rubbish_key.pem -out ${REPOSITORY_ROOT_DIR}/test/credentials/pg_rubbish_cert.pem  -config ${STACKQL_CORE_DIR}/test/server/mtls/openssl.cnf -days 365