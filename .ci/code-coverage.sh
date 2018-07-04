#!/usr/bin/env bash

# Copyright 2018 The go-interpreter Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


set -e

echo "" > coverage.txt

for d in $(go list ./... | grep -v vendor); do
    go test $TAGS -race -coverprofile=profile.out -covermode=atomic $d
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
done
