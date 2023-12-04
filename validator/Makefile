DOCKER := $(shell which docker)

build-docker:
	$(DOCKER) build --secret id=sshKey,src=${BBN_PRIV_DEPLOY_KEY} --tag babylonchain/btc-validator -f Dockerfile \
		$(shell git rev-parse --show-toplevel)

.PHONY: build-docker

###############################################################################
###                                Protobuf                                 ###
###############################################################################

proto-all: proto-gen

proto-gen:
	./proto/scripts/protocgen.sh

.PHONY: proto-gen
