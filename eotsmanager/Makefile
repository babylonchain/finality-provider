BUILDDIR ?= $(CURDIR)/build

DOCKER := $(shell which docker)
CUR_DIR := $(shell pwd)

ldflags := $(LDFLAGS)
build_tags := $(BUILD_TAGS)
build_args := $(BUILD_ARGS)

ifeq ($(LINK_STATICALLY),true)
	ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static" -v
endif

ifeq ($(VERBOSE),true)
	build_args += -v
endif

BUILD_TARGETS := build install
BUILD_FLAGS := --tags "$(build_tags)" --ldflags '$(ldflags)'

all: build install

build: BUILD_ARGS := $(build_args) -o $(BUILDDIR)

clean:
	rm -rf $(BUILDDIR)

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

build-docker:
	$(DOCKER) build --secret id=sshKey,src=${BBN_PRIV_DEPLOY_KEY} --tag babylonchain/eots-manager -f Dockerfile \
		$(shell git rev-parse --show-toplevel)

.PHONY: build build-docker clean

###############################################################################
###                                Protobuf                                 ###
###############################################################################

proto-all: proto-gen

proto-gen:
	./proto/scripts/protocgen.sh

.PHONY: proto-gen
