#!/bin/bash

# echo "cmd$: go build"
# go build
# echo -e ""

echo "run client"
./clientsample -conns 50 -c 40000 -t 40000
