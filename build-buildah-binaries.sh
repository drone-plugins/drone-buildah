#!/usr/bin/env bash

docker build --build-arg=GIT_USER_NAME="$(git config user.name)" --build-arg=GIT_USER_EMAIL="$(git config user.email)" \
    -f Dockerfile-build-buildah -t buildah-dev .

mkdir buildah-binaries

docker container create --name extract buildah-dev  
docker container cp extract:/root/buildah/src/github.com/containers/buildah/bin ./buildah-binaries
docker container rm -f extract