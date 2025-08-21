#!/bin/bash -eux
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

pushd dis-bundle-scheduler
  make lint
popd