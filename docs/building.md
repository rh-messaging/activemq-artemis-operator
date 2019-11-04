# Building the operator

- [imagebuilder](https://github.com/openshift/imagebuilder)

```$xslt
imagebuilder -t activemq-artemis-operator:latest -f build/Dockerfile .
```

or

```$xslt
imagebuilder -t activemq-artemis-operator:latest-debug -f build/Dockerfile_debug .
```

# Running tests

The tests are under test directory. The tests are written using 
[Qpid Dispatch Go Test Framework](https://github.com/fgiorgetti/qpid-dispatch-go-tests)

To run tests you need to install the dependency into your GOPATH.

```$xslt
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...
go get github.com/fgiorgetti/qpid-dispatch-go-tests/pkg/framework
```

Then run test with the following command:

```$xslt
ginkgo -r test
```

It will run all tests under test dir.

More options for ginkgo command are available here
(https://onsi.github.io/ginkgo/#the-ginkgo-cli)
