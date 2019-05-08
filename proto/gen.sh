#!/bin/sh

INC_PATH="-I. -I$GOPATH/src/ -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway"

protoc $INC_PATH --go_out=plugins=grpc:../pb/ *.proto
protoc $INC_PATH --grpc-gateway_out=logtostderr=true:../pb *.proto
protoc $INC_PATH --swagger_out=logtostderr=true:../docs *.proto
