#! /bin/bash
set -e

go run golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@v0.49.0 "$@"
