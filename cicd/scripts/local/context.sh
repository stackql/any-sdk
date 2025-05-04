#!/usr/bin/env bash


SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

export REPOSITORY_ROOT_DIR="$(realpath ${SCRIPT_DIR}/../../..)"

export STACKQL_CORE_DIR="${STACKQL_CORE_DIR:-"${REPOSITORY_ROOT_DIR}/stackql-core}"}"


checkPoetry () {
    if ! command -v poetry &> /dev/null
    then
        >&2 echo "Poetry is not installed. Please install it first." 
        exit 1
    fi
}
