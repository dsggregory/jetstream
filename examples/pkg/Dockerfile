# Use this when making local docker images and/or to push to our repo.
# It needs your SSH ID key so it can connect to bitbucket during `go get` of our packages.
FROM golang:alpine AS build-env
ENV GO111MODULE=on
# fixes a go v1.16 issue where it complains about a missing go.sum entry and instructs you to do a `go get ...`
ENV GOFLAGS=-mod=mod
RUN apk add --no-cache git build-base && \
    echo "http://dl-cdn.alpinelinux.org/alpine/edge/main" >> /etc/apk/repositories && \
    echo "http://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories && \
    echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories && \
    apk add --no-cache upx openssh

RUN mkdir -p /go/src/jetstream-test/app
WORKDIR /go/src/jetstream-test
COPY ../.. .
WORKDIR /go/src/jetstream-test/examples/pkg
RUN make build

# final stage
FROM alpine
# FROM gcr.io/distroless/static-debian10
RUN apk update && \
    apk add ca-certificates && \
    rm -rf /var/cache/apk/* && \
    rm -rf /tmp/*
WORKDIR /app

COPY --from=build-env /go/src/jetstream-test/examples/pkg/app/jetstream-test /app/
CMD [ "cp", "/app/jetstream-test", "/dist" ]
