#!/usr/bin/env bash

set -e

go version

# set output format for linter
if [[ -v CI ]]; then
    OUT_FORMAT="github-actions"
else
    OUT_FORMAT="colored-line-number"
fi

golangci-lint run --disable-all --deadline=10m \
    --out-format=$OUT_FORMAT \
    --enable=gofmt \
    --enable=govet \
    --enable=ineffassign \
    --enable=revive \
    --enable=vetshadow \
    --enable=unconvert \
    --enable=goimports \
    --enable=misspell \
    --enable=asciicheck \
    --enable=unparam \
    --enable=unused \
    --enable=gosimple \
    --enable=goconst
