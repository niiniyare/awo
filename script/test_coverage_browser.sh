#!/bin/bash

rm -rf ~/storage/shared/Download/coverage.html

go test -v -cover -coverprofile=coverage.out ./db/sqlc/...

go tool cover -html=coverage.out

lsd -t /data/data/com.termux/files/usr/tmp/ >./script/coverage.txt

file=/data/data/com.termux/files/usr/tmp/$(head -n+1 ./script/coverage.txt)/coverage.html

termux-open $file
# $HOME/awo/db/sqlc
