####################
### Build Binaries #
####################

FROM golang:1.21.1 AS build-env

# Version to build. Default is the Git HEAD.
ARG VERSION="HEAD"

# Use muslc for static libs
ARG BUILD_TAGS="muslc"

RUN apt-get update && \
  apt-get --no-install-recommends --yes install \
  git-core \
  wget \
  bzip2 \
  jq \
  vim \
  ca-certificates \
  build-essential \
  pkg-config \
  cmake \
  gcc


# Cosmwasm - download correct libwasmvm version
# fp binaries are dependent on this package to run
RUN git clone --branch v0.1.0 https://github.com/babylonchain/finality-provider.git && \
  cd finality-provider && \
  go mod download && \
  WASMVM_VERSION=$(go list -m github.com/CosmWasm/wasmvm | cut -d ' ' -f 2) && \
  wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm_muslc.$(uname -m).a \
  -O /lib/libwasmvm_muslc.a && \
  # verify checksum
  wget https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/checksums.txt -O /tmp/checksums.txt && \
  sha256sum /lib/libwasmvm_muslc.a | grep $(cat /tmp/checksums.txt | grep libwasmvm_muslc.$(uname -m) | cut -d ' ' -f 1)

RUN cd finality-provider/ && \
  CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
  CGO_ENABLED=1 \
  BUILD_TAGS=$BUILD_TAGS \
  LINK_STATICALLY=true && \
  make build


# ###########################
# ### Finality Provider Run #
# ###########################

FROM debian:bookworm-slim AS run-env

RUN addgroup --gid  1138 --system finality-provider && \
  adduser --uid  1138 finality-provider --ingroup finality-provider

# Copy the necessary binaryies from previous steps
COPY --from=build-env go/finality-provider/build/eotsd /bin/eotsd
COPY --from=build-env go/finality-provider/build/fpd /bin/fpd
COPY --from=build-env go/finality-provider/build/fpcli /bin/fpcli

WORKDIR /home/finality-provider
RUN chown -R finality-provider /home/finality-provider
USER finality-provider