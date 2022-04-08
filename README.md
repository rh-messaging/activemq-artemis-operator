# ActiveMQ Artemis Operator

This project is a [Kubernetes](https://kubernetes.io/) [operator](https://coreos.com/blog/introducing-operators.html)
to manage the [Apache ActiveMQ Artemis](https://activemq.apache.org/artemis/) message broker.

## Status ##

The current api version of all main CRDs managed by the operator is **v1beta1**.

## Quickstart

The [quickstart.md](docs/quickstart.md) provides simple steps to quickly get the operator up and running
as well as deploy/managing broker deployments.

## Building

The [building.md](docs/building.md) describes how to build operator and how to test your changes

## OLM integration

The [bundle.md](docs/bundle.md) contains instructions for how to build operator bundle images and integrate it into [Operator Liftcycle Manager](https://olm.operatorframework.io/) framework.

