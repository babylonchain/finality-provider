#!/usr/bin/env bash

set -eo pipefail

cd proto
buf mod update
buf generate .
cd ..
