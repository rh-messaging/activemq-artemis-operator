# kernel-style V=1 build verbosity
ifeq ("$(origin V)", "command line")
       BUILD_VERBOSE = $(V)
endif

ifeq ($(BUILD_VERBOSE),1)
       Q =
else
       Q = @
endif




#export CGO_ENABLED:=0

.PHONY: all
all: build

.PHONY: dep
dep:
	./scripts/go-dep.sh

.PHONY: format
format:
	./scripts/go-fmt.sh

.PHONY: go-generate
go-generate: dep
	$(Q)go generate ./...

.PHONY: sdk-generate
sdk-generate: dep
	operator-sdk generate k8s

.PHONY: vet
vet:
	./scripts/go-vet.sh

.PHONY: test
test:
	./scripts/go-test.sh

.PHONY: lint
lint:
	# Temporarily disabled
	# ./scripts/go-lint.sh
	# ./scripts/yaml-lint.sh

.PHONY: build
build:
	./scripts/go-build.sh


.PHONY: clean
clean:
	rm -rf build/_output


.PHONY: csv
csv:
	./scripts/go-csv.sh


# test/ci-go: test/sanity test/unit test/subcommand test/e2e/go

# test/ci-ansible: test/e2e/ansible

# test/sanity:
# 	./scripts/tests/sanity-check.sh

# test/unit:
# 	./scripts/tests/unit.sh

# test/subcommand:
# 	./scripts/tests/test-subcommand.sh

# test/e2e: test/e2e/go test/e2e/ansible

# test/e2e/go:
# 	./scripts/tests/e2e-go.sh

# test/e2e/ansible:
# 	./scripts/tests/e2e-ansible.sh
