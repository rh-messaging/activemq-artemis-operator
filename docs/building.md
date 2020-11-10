# Building the operator

Commonly used commands:

 - `go build -v -o operator ./cmd/manager` for building operator
 - `podman build -f ./build/Dockerfile -t amq-broker-operator:latest .` for building operator image
