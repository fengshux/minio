#!/bin/bash

GOOS=linux GOARCH=amd64 go build -o build/s3m s3m .
