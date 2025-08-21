#!/bin/bash -eux

pushd dis-bundle-scheduler
  make build
  cp build/dis-bundle-scheduler Dockerfile.concourse ../build
popd
