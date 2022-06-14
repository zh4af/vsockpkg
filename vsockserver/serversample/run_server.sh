#!/bin/bash

echo "cmd$: go build"
go build
echo -e ""

chmod a+x serversample
./serversample
