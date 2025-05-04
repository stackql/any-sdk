#! /usr/bin/env bash

>&2 echo "requires git version >= 2.45"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/context.sh"

cd "${REPOSITORY_ROOT_DIR}"

git clone --revision=refs/heads/main  https://github.com/stackql/stackql.git stackql-core

