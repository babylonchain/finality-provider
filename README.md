# BTC-Validator

Before you start using BTC-Validator, make sure to follow these steps to download, build, and set up the program:

1. **Installation guide:** Follow the [installation guide](docs/installation.md).

2. **Configuration:** Follow the  [configuration guide](docs/configuration.md).

3. **Running the Program:** Follow the [operations guide](docs/operations.md).

## Overview

BTC-Validator is a standalone program crafted for the creation and management of BTC validators. 
The program includes a CLI functionality for the creation, management, and storage of validator keys, as well as the creation and registration of validators on the consumer chain.

Once a validator is registered on the chain, BTC-Validator consistently polls for new blocks. It actively engages 
with the blockchain by sending finality signatures and committing public randomness at regular intervals.

The program consists of two essential components: the **EOTS manager Daemon** and the **Validator Daemon**.

### 1. EOTS Manager Daemon
The EOTS (Extractable One Time Signature) Daemon is responsible for managing EOTS keys, producing EOTS randomness 
and EOTS signatures

1. **EOTS Key Management:**
    - Generates [Schnorr](https://en.wikipedia.org/wiki/Schnorr_signature) key pairs for the validator using the 
      [BIP-340](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki) standard.
    - Persists generated key pairs in internal storage([bolt db](https://github.com/etcd-io/bbolt)).

2. **Randomness Generation:**
    - Generates lists of [Schnorr randomness pairs](https://www.researchgate.net/publication/222835548_Schnorr_Randomness) based on the EOTS key, chainID, and block height.
    - The randomness is deterministically generated and tied to specific parameters.

3. **Signature Generation:**
    - Signs EOTS using the private key of the validator and corresponding secret randomness for a given chain at a specified height.
    - Signs Schnorr signatures using the private key of the validator.

### 2. Validator Daemon

The Validator Daemon is responsible for finality signatures and randomness commitment.

1. **Finality Signatures:**
    - Sends the finality signature to the consumer chain (Babylon) for each registered validator and for each block.

2. **Randomness Commitment:**
    - Ensures the inclusion of public randomness in each block it processes.
