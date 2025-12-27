#!/bin/bash
go get -u all
go build -o api -ldflags "-s -w" src/main.go