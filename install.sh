#!/bin/bash
BASE_PATH=`pwd`
VERSION="v.3.1"

if [[ "$(whoami)" != "root" ]]
then
    echo "sorry, you are not root"
    exit 1
fi

if [[ -z "$(which git)" ]]
then
    echo "sorry, git are not installed"
    exit 1
fi

if [[ -z "$(which go)" ]]
then
    echo "sorry, go are not installed"
    exit 1
fi

export GOPATH="$BASE_PATH"
export GOBIN="$BASE_PATH/bin/"
go get -d github.com/actionpay/postmanq/cmd
cd "$BASE_PATH/src/github.com/actionpay/postmanq"
git checkout "$VERSION"
go install cmd/postmanq.go
go install cmd/pmq-grep.go
go install cmd/pmq-publish.go
go install cmd/pmq-report.go
ln -s "$BASE_PATH/bin/postmanq" /usr/bin/
ln -s "$BASE_PATH/bin/pmq-grep" /usr/bin/
ln -s "$BASE_PATH/bin/pmq-publish" /usr/bin/
ln -s "$BASE_PATH/bin/pmq-report" /usr/bin/