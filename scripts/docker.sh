#!/bin/sh

# force go modules
export GOPATH=""

# disable cgo
export CGO_ENABLED=0

# force linux amd64 platform
export GOOS=linux
export GOARCH=amd64 

set -e
set -x

# build the binary
GOOS=linux GOARCH=amd64 go build -o release/linux/amd64/drone-gcr ./cmd/drone-gcr
GOOS=linux GOARCH=amd64 go build -o release/linux/amd64/drone-ecr ./cmd/drone-ecr
GOOS=linux GOARCH=amd64 go build -o release/linux/amd64/drone-docker ./cmd/drone-docker
GOOS=linux GOARCH=amd64 go build -o release/linux/amd64/drone-acr ./cmd/drone-acr
GOOS=linux GOARCH=amd64 go build -o release/linux/amd64/drone-heroku ./cmd/drone-heroku

# build the docker image
docker build -f docker/gcr/Dockerfile.linux.amd64 -t plugins/buildah-gcr .
docker build -f docker/ecr/Dockerfile.linux.amd64 -t plugins/buildah-ecr .
docker build -f docker/docker/Dockerfile.linux.amd64 -t plugins/buildah-docker .
docker build -f docker/docker-nonroot/Dockerfile.linux.amd64 -t plugins/buildah-docker .
docker build -f docker/acr/Dockerfile.linux.amd64 -t plugins/buildah-acr .
docker build -f docker/heroku/Dockerfile.linux.amd64 -t plugins/buildah-heroku .
