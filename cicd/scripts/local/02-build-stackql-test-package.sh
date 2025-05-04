#!/usr/bin/env bash

>&2 echo "requires all of requirements.txt"

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/context.sh"

PACKAGE_ROOT="${STACKQL_CORE_DIR}/test"

rm -f ${PACKAGE_ROOT}/dist/*.whl || true

${STACKQL_CORE_DIR}/cicd/util/01-build-robot-lib.sh

filez="$(ls ${PACKAGE_ROOT}/dist/*.whl)" || true

if [ "${filez}" = "" ]; then
    >&2 echo "No wheel files found in ${PACKAGE_ROOT}/dist. Please check the build process."
    exit 1
else
    echo "Wheel files found in ${PACKAGE_ROOT}/dist: ${filez}"
fi




