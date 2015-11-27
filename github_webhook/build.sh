#!/bin/sh

gopath=${PWD}
export GOPATH=$gopath
echo "GOPATH set to $GOPATH"

go get github.com/golang/glog

go build

