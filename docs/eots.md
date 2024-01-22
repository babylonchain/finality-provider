# EOTS Manager

## 1. Overview

The EOTS daemon is responsible for managing EOTS keys,
producing EOTS randomness, and using them to produce EOTS signatures.

**Note:** EOTS stands for Extractable One Time Signature. You can read more about it
in
the [Babylon BTC Staking Litepaper](https://docs.babylonchain.io/assets/files/btc_staking_litepaper-32bfea0c243773f0bfac63e148387aef.pdf).
In short, the EOTS manager produces EOTS public/private randomness pairs.
The finality provider commits the public part of this pairs to Babylon for
every future block height that they intend to provide a finality signature for.
If the finality provider votes for two different blocks on the same height,
they will have to reuse the same private randomness which will lead to their
underlying private key being exposed, leading to the slashing of them and all their delegators.

The EOTS manager is responsible for the following operations:
1. **EOTS Key Management:**
    - Generates [Schnorr](https://en.wikipedia.org/wiki/Schnorr_signature) key pairs
      for a given finality provider using the
      [BIP-340](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki)
      standard.
    - Persists generated key pairs in the
      internal Cosmos keyring.
2. **Randomness Generation:**
    - Generates lists of EOTS randomness pairs based on the EOTS key, chainID, and
      block height.
    - The randomness is deterministically generated and tied to specific parameters.
3. **Signature Generation:**
    - Signs EOTS using the private key of the finality provider and the corresponding secret
      randomness for a given chain at a specified height.
    - Signs Schnorr signatures using the private key of the finality provider.

The EOTS manager functions as a daemon controlled by the `eotsd` tool.

## 2. Configuration

The `eotsd init` command initializes a home directory for the EOTS
manager. This directory is created in the default home location or in a location
specified by the `--home` flag.

```bash
eotsd init --home /path/to/eotsd/home/
```

After initialization, the home directory will have the following structure

```bash
ls /path/to/eotsd/home/
  ├── eotsd.conf # Eotsd-specific configuration file.
  ├── logs       # Eotsd logs
```

If the `--home` flag is not specified, then the default home location will
be used. For different operating systems, those are:

- **MacOS** `~/Users/<username>/Library/Application Support/Eotsd`
- **Linux** `~/.Eotsd`
- **Windows** `C:\Users\<username>\AppData\Local\Eotsd`

Below are the `eotsd.conf` file contents:

```bash
# Default address to listen for RPC connections
RpcListener = 127.0.0.1:15813

# Type of keyring to use,
# supported backends - (os|file|kwallet|pass|test|memory)
# ref https://docs.cosmos.network/v0.46/run-node/keyring.html#available-backends-for-the-keyring
KeyringBackend = file

# Possible database to choose as backend
Backend = bbolt

# Path to the database
Path = bbolt-eots.db

# Name of the database
Name = default
```

## 3. Starting the EOTS Daemon

You can start the EOTS daemon using the following command:

```bash
eotsd start --home /path/to/eotsd/home
```

This will start the rpc server at the address specified in the configuration under
the `RpcListener` field, which has a default value of `127.0.0.1:15813`.
You can also specify a custom address using the `--rpc-listener` flag.

```bash
eotsd start

time="2023-11-26T16:35:04-05:00" level=info msg="RPC server listening	{"address": "127.0.0.1:15813"}"
time="2023-11-26T16:35:04-05:00" level=info msg="EOTS Manager Daemon is fully active!"
```

All the available cli options can be viewed using the `--help` flag. These options
can also be set in the configuration file.

**Note**: It is recommended to run the `eotsd` daemon on a separate machine or
network segment to enhance security. This helps isolate the key management
functionality and reduces the potential attack surface. You can edit the
`EOTSManagerAddress` in  the configuration file of the finality provider
to reference the address of the machine where `eotsd` is running.
