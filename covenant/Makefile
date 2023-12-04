DOCKER := $(shell which docker)

build-docker:
	$(DOCKER) build --secret id=sshKey,src=${BBN_PRIV_DEPLOY_KEY} --tag babylonchain/covenant -f Dockerfile \
		$(shell git rev-parse --show-toplevel)

.PHONY: build-docker
