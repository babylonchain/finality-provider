#!/usr/bin/env bash

set -eo pipefail

cd eotsmanager/proto
buf mod update
buf generate .
cd ../..

go mod tidy -compat=1.20
