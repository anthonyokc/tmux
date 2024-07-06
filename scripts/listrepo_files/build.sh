#!/bin/bash
go get -d ./...
go build -ldflags="-s -w" -o ../listrepo listrepo.go
go build -ldflags="-s -w" -o ../listrepo_gql listrepo_gql.go
