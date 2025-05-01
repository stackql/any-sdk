#!/usr/bin/env bash

>&2 echo "requires all of requirements.txt"

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/context.sh"

cd "${REPOSITORY_ROOT_DIR}"

source "${REPOSITORY_ROOT_DIR}/.venv/bin/activate"

export PYTHONPATH="${PYTHONPATH}:${REPOSITORY_ROOT_DIR}/test/python"

robot -d test/robot/reports/mocked test/robot/cli/mocked

