# Finality Provider

## 1. Overview

The Finality Provider Daemon is responsible for
monitoring for new Babylon blocks,
committing public randomness for the blocks it
intends to provide finality signatures for, and
submitting finality signatures.

The daemon can manage and perform the following operations for multiple
finality providers:
1. **Creation and Registration**: Creates and registers finality 
   providers to Babylon.
2. **EOTS Randomness Commitment**: The daemon monitors the Babylon chain and
   commits EOTS public randomness for every Babylon block each
   finality provider intends to vote for. The commit intervals can be specified
   in the configuration.
   The EOTS public randomness is retrieved through the finality provider daemon's
   connection with the [EOTS daemon](eots.md).
3. **Finality Votes Submission**: The daemon monitors the Babylon chain
   and produces finality votes for each block each maintained finality provider
   has committed to vote for.

The daemon is controlled by the `fpd` tool.
The `fpcli` tool implements commands for interacting with the daemon.

## 2. Configuration

The `fpd init` command initializes a home directory for the
finality provider daemon.
This directory is created in the default home location or in a
location specified by the `--home` flag.

```bash
$ fpd init --home /path/to/fpd/home/
```

After initialization, the home directory will have the following structure

```bash
$ ls /path/to/fpd/home/
  ├── fpd.conf # Fpd-specific configuration file.
  ├── logs     # Fpd logs
```

If the `--home` flag is not specified, then the default home directory
will be used. For different operating systems, those are:

- **MacOS** `~/Library/Application Support/Fpd`
- **Linux** `~/.Fpd`
- **Windows** `C:\Users\<username>\AppData\Local\Fpd`

Below are some important parameters of the `fpd.conf` file.

**Note**:
The finality provider daemon requires the existence of a keyring that contains
an account with Babylon token funds to pay for transactions.
The configuration below requires to point to the path where this keyring is stored
and specify the account name under the `KeyDirectory` and `Key` config values respectively.

```bash
# Address of the EOTS Daemon
EOTSManagerAddress = 127.0.0.1:15813

# Babylon specific parameters

# Babylon chain ID
ChainID = chain-test

# Babylon node RPC endpoint
RPCAddr = http://localhost:26657

# Babylon node gRPC endpoint
GRPCAddr = https://localhost:9090

# Name of the key in the keyring to use for signing transactions
Key = <finality-provider-key-name>

# Type of keyring to use,
# supported backends - (os|file|kwallet|pass|test|memory)
# ref https://docs.cosmos.network/v0.46/run-node/keyring.html#available-backends-for-the-keyring
KeyringBackend = test

# Directory where keys will be retrieved from and stored
KeyDirectory = /path/to/fpd/home
```

To see the complete list of configuration options, check the `fpd.conf` file.

## 3. Starting the Finality Provider Daemon

You can start the finality provider daemon using the following command:

```bash
$ fpd start --home /path/to/fpd/home
```

This will start the RPC server at the address specified in the configuration under
the `RawRPCListeners` field. A custom address can also be specified using
the `--rpclisten` flag.

```bash
$ fpd start --rpclisten 'localhost:8082'

time="2023-11-26T16:37:00-05:00" level=info msg="successfully connected to a remote EOTS manager at 127.0.0.1:8081"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting Finality Provider App"
time="2023-11-26T16:37:00-05:00" level=info msg="Version: 0.2.2-alpha commit=, build=production, logging=default, debuglevel=info"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting RPC Server"
time="2023-11-26T16:37:00-05:00" level=info msg="RPC server listening on 127.0.0.1:8082"
time="2023-11-26T16:37:00-05:00" level=info msg="Finality Provider Daemon is fully active!"
```

All the available CLI options can be viewed using the `--help` flag. These options
can also be set in the configuration file.

## 4. Create and Register a Finality Provider

A finality provider named `my-finality-provider` can be created in the internal
storage ([bolt db](https://github.com/etcd-io/bbolt))
through the `fpcli create-finality-provider` command.
This finality provider is associated with a BTC public key which
serves as its unique identifier and
a Babylon account to which staking rewards will be directed.

```bash
$ fpcli create-finality-provider --key-name my-finality-provider \
                --chain-id chain-test --passphrase mypassphrase
{
    "btc_pk": "903fab42070622c551b188c983ce05a31febcab300244daf7d752aba2173e786"
}
```

The finality provider can be registered with Babylon through
the `register-finality-provider` command.
The output contains the hash of the Babylon
finality provider registration transaction.

```bash
$ fpcli register-finality-provider \
                 --btc-pk 903fab42070622c551b188c983ce05a31febcab300244daf7d752aba
{
    "tx_hash": "800AE5BBDADE974C5FA5BD44336C7F1A952FAB9F5F9B43F7D4850BA449319BAA"
}
```

To verify that your finality provider has been created,
we can check the finality providers that are managed by the daemon and their status.
These can be listed through the `fpcli list-finality-providers` command.
The `status` field can receive the following values:

- `1`: The finality provider is active and has received no delegations yet
- `2`: The finality provider is active and has staked BTC tokens
- `3`: The finality provider is inactive (i.e. had staked BTC tokens in the past but not
  anymore OR has been slashed)
 
The `last_committed_height` field is the Babylon height up to which the finality provider
has committed EOTS randomness

```bash
$ fpcli list-finality-providers
{
    "finality-providers": [
        ...
        {
            "babylon_pk_hex": "0251259b5c88d6ac79d86615220a8111ebb238047df0689357274f004fba3e5a89",
            "btc_pk_hex": "903fab42070622c551b188c983ce05a31febcab300244daf7d752aba2173e786",
            "last_committed_height": 265,
            "status": 1
        }
    ]
}
```
