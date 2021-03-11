#!/usr/bin/env bash
# Stops the process if something fails
set -xe
sudo rm /var/app/current/go.*
echo $GOPATH
ls $GOPATH
go get
mkdir -p /var/app/current/bin/custom/conf
# create the application binary that eb uses
GOOS=linux GOARCH=amd64 go build -o bin/application application.go
ls
sudo cp custom/conf/app.ini /var/app/current/bin/custom/conf/app.ini
