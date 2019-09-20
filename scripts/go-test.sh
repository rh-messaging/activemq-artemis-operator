#!/bin/sh

if [[ -z ${CI} ]]; then
    ./scripts/go-vet.sh
    ./scripts/go-fmt.sh
    ./scripts/catalog-source.sh
fi
go test ./...