# BTC-Validator

## 1. Overview

BTC-Validator is a standalone program crafted for the creation and management of BTC
validators. The program includes a CLI functionality for the creation, management,
and storage of validator keys, as well as the creation and registration of validators
on the consumer chain.

Once a validator is registered on the chain, BTC-Validator consistently polls for new
blocks. It actively engages with the blockchain by sending finality signatures and
committing public randomness at regular intervals.

The program consists of two essential components: the **EOTS manager Daemon** and the
**Validator Daemon**.

#### 1. EOTS Manager Daemon

The EOTS Daemon is responsible for managing EOTS keys, producing EOTS randomness and
EOTS signatures

**Note:** EOTS stands for Extractable One Time Signature. You can read more about it
in
the [Babylon BTC Staking Litepaper](https://docs.babylonchain.io/assets/files/btc_staking_litepaper-32bfea0c243773f0bfac63e148387aef.pdf).

1. **EOTS Key Management:**
    - Generates [Schnorr](https://en.wikipedia.org/wiki/Schnorr_signature) key pairs
      for the validator using the
      [BIP-340](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki)
      standard.
    - Persists generated key pairs in the
      internal [bolt db](https://github.com/etcd-io/bbolt) storage.

2. **Randomness Generation:**
    - Generates lists
      of [Schnorr randomness pairs](https://www.researchgate.net/publication/222835548_Schnorr_Randomness)
      based on the EOTS key, chainID, and block height.
    - The randomness is deterministically generated and tied to specific parameters.

3. **Signature Generation:**
    - Signs EOTS using the private key of the validator and corresponding secret
      randomness for a given chain at a specified height.
    - Signs Schnorr signatures using the private key of the validator.

#### 2. Validator Daemon

The Validator Daemon is responsible for finality signatures and randomness
commitment.

1. **Finality Signatures:**
    - Sends the finality signature to the consumer chain (Babylon) for each
      registered validator and for each block there's an EOTS randomness commitment.

2. **EOTS Randomness Commitment:**
    - Ensures the generation of EOTS randomness commitment on the Babylon ledger for
      each block the BTC validator intends to vote for.

## 2. Installation

#### Prerequisites

This project requires Go version 1.20 or later.

Install Go by following the instructions on
the [official Go installation guide](https://golang.org/doc/install).

#### Downloading the code

To get started, clone the repository to your local machine from Github:

```bash
$ git clone git@github.com:babylonchain/btc-validator.git
```

You can choose a specific version from
the [official releases page](https://github.com/babylonchain/btc-validator/releases)

```bash
$ cd btc-validator # cd into the project directory
$ git checkout <release-tag>
```

#### Building and installing the binary

```bash
# cd into the project directory
$ cd btc-validator 

# installs the compiled binaries to your
# $GOPATH/bin directory allowing access
# from anywhere on your system
$ make install 
```

The above will produce the following binaries:

- `eotsd`: The daemon program for the EOTS manager.
- `eotcli`: The CLI tool for interacting with the EOTS daemon.
- `vald`: The daemon program for the btc-validator.
- `valcli`: The CLI tool for interacting with the btc-validator daemon.

To build locally,

```bash
$ cd btc-validator # cd into the project directory
$ make build
```

The above will lead to a build directory having the following structure:

```bash
$ ls build
    ├── eotcli
    ├── eotsd
    ├── valcli
    └── vald
```

## 3. Configuration

Follow the  [configuration guide](docs/configuration.md).

## 4. Running the BTC Validator Program

Follow the [operations guide](docs/operation.md).
