#!/bin/bash

# This is a comment
echo "run bash with lmstudio"

go run main.go \
-endpoint="http://localhost:1234/v1/" \
-ollama=false                         \
-context=100000                       \
-input=example.txt                    