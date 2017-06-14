#!/usr/bin/env bash

if [ ! -f "$0" ]; then
    echo 'make must be run within its container folder' 1>&2
    exit 1
fi

if [ ! -d "$GOROOT" ]; then
	GOROOT="/usr/local/go"
fi

CURDIR=`pwd`
export GOROOT=$GOROOT
export GOPATH=$CURDIR/../../../
echo $GOPATH

svnurl=`git config -l| grep remote.origin.url |awk -F "=" '{print $2}'`
svnver=`git rev-parse HEAD`
author=`git config -l| grep user.email |awk -F "=" '{print $2}'`
date=`date "+%Y-%m-%d_%H:%M:%S"`

#goversion=`$GOROOT/bin/go version`

ldflags="-X model._SVN_=$svnurl -X model._BASE_VERSION_=$svnver -X model._AUTHOR_=$author -X model._COMPILE_TIME_=\"$date\""
echo ldflags $ldflags

echo "formating code..."
$GOROOT/bin/gofmt -w ./

# 以下命令可以使用go get golang.org/x/tools/cmd/goimports获取
goimports -w=true ./

echo "build tools"
$GOROOT/bin/go build -o ../../bin/tools/protoc-gen-go lucifinil-long/nano-legion/vendor/github.com/golang/protobuf/protoc-gen-go

echo "building administrative center and agent..."

echo "building components..."

echo "building unittests..."

echo 'finished'
