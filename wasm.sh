#!/bin/bash -x

cd "$(dirname "$0")" || exit

trap 'exit 130' INT
while true; do
	find ./cmd/wasm |
		entr -dsr "GOOS=js GOARCH=wasm go build -o ./cmd/wasm/main.wasm ./cmd/wasm && \
go run ./cmd/httpserver $(realpath ./cmd/wasm)"
done
