#!/usr/bin/env bash

_curDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

_repositoryRootDir="$(realpath $_curDir/../..)"

go get ./...

go build -o build/anysdk ./cmd/interrogate

