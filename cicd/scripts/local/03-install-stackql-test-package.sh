#!/usr/bin/env bash

>&2 echo "requires all of requirements.txt"

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/context.sh"

if [ ! -d "${REPOSITORY_ROOT_DIR}/.venv" ]; then
  >&2 echo "No existing virtual environment, creating one..."
  >&2 echo "Creating virtual environment in ${REPOSITORY_ROOT_DIR}/.venv"
  python -m venv "${REPOSITORY_ROOT_DIR}/.venv"
  >&2 echo "Virtual environment created."
else
  >&2 echo "Using existing virtual environment in ${REPOSITORY_ROOT_DIR}/.venv"
fi

source "${REPOSITORY_ROOT_DIR}/.venv/bin/activate"

pip install -r "${REPOSITORY_ROOT_DIR}/cicd/testing-requirements.txt"

PACKAGE_ROOT="${STACKQL_CORE_DIR}/test"

filez="$(ls ${PACKAGE_ROOT}/dist/*.whl)" || true

if [ "${filez}" = "" ]; then
    >&2 echo "No wheel files found in ${PACKAGE_ROOT}/dist. Please check the build process."
    exit 1
else
    echo "Wheel files found in ${PACKAGE_ROOT}/dist: ${filez}"
fi


for file in ${PACKAGE_ROOT}/dist/*.whl; do
    pip3 install "$file" --force-reinstall
done
