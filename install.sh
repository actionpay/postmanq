#!/bin/bash

if [[ "$(whoami)" != "root" ]]
then
    echo "sorry, you are not root"
    exit 1
fi

if [[ -z "$(which git)" ]]
then
    echo "git are not installed!"
    exit 2
fi

if [[ -z "$(which go)" ]]
then
    echo "go are not installed!"
    exit 3
fi

BASE_PATH=`pwd`
VERSION="v.3.1"
export GOPATH="$BASE_PATH"
export GOBIN="$BASE_PATH/bin/"
go get -d github.com/Halfi/postmanq/cmd
cd "$BASE_PATH/src/github.com/Halfi/postmanq"
git checkout "$VERSION"
go install cmd/postmanq.go
go install cmd/pmq-grep.go
go install cmd/pmq-publish.go
go install cmd/pmq-report.go
ln -s "$BASE_PATH/bin/postmanq" /usr/bin/
ln -s "$BASE_PATH/bin/pmq-grep" /usr/bin/
ln -s "$BASE_PATH/bin/pmq-publish" /usr/bin/
ln -s "$BASE_PATH/bin/pmq-report" /usr/bin/