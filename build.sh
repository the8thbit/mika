#!/bin/bash

echo "Using gopath: $GOPATH"
if [ "$1" == "-u" ]
then
    deps=(
        "golang.org/x/tools/cmd/godoc|master"
        "golang.org/x/tools/cmd/vet|master"
        "github.com/golang/lint/golint|master"
        "golang.org/x/tools/cmd/cover|master"
        "github.com/chihaya/bencode|master"
        "github.com/garyburd/redigo/redis|master"
        "github.com/kisielk/raven-go/raven|master"
        "github.com/labstack/echo|v0.0.12"
        "github.com/julienschmidt/httprouter|master"
        "github.com/rcrowley/go-metrics/influxdb|master"
        "github.com/Sirupsen/logrus|master"
        "github.com/goji/param|master"
        "github.com/influxdb/influxdb/client|v0.8.8"
        "github.com/goji/httpauth|master"
    )
    pushd &> /dev/null
    for dep in "${deps[@]}"
    do
        repo=$(echo ${dep} | cut -f1 -d\|)
        version=$(echo ${dep} | cut -f2 -d\|)

        echo "Cloning $repo..."
        go get ${repo}

        pushd ${GOPATH}/src/${repo} &> /dev/null
            echo "Checking out $repo @ $version"
            git fetch
            git pull origin ${version}
            git checkout ${version}
        popd &> /dev/null
    done
fi
make && go vet && make test
