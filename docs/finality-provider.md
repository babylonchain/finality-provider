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

- **MacOS** `~/Users/<username>/Library/Application Support/Fpd`
- **Linux** `~/.Fpd`
- **Windows** `C:\Users\<username>\AppData\Local\Fpd`

Below are some important parameters of the `fpd.conf` file.

**Note**:
The configuration below requires to point to the path where this keyring is stored `KeyDirectory`.
This `Key` field stores the key name used for interacting with the consumer chain
and will be specified along with the `KeyringBackend` field in the next [step](#3-add-key-for-the-consumer-chain).
So we can ignore the setting of the two fields in this step.

```bash
# RPC Address of the EOTS Daemon
EOTSManagerAddress = 127.0.0.1:15813

# Babylon specific parameters

# Babylon chain ID
ChainID = chain-test

# Babylon node RPC endpoint
RPCAddr = http://127.0.0.1:26657

# Babylon node gRPC endpoint
GRPCAddr = https://127.0.0.1:9090

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

## 3. Add key for the consumer chain

The finality provider daemon requires the existence of a keyring that contains
an account with Babylon token funds to pay for transactions.
This key will be also used to pay for fees of transactions to the consumer chain.

Use the following command to add the key:

```bash
$ fpd keys add --key-name my-finality-provider --chain-id chain-test
```

After executing the above command, the key name will be saved in the config file
created in [step](#2-configuration).

## 4. Starting the Finality Provider Daemon

You can start the finality provider daemon using the following command:

```bash
$ fpd start --home /path/to/fpd/home
```

This will start the RPC server at the address specified in the configuration under
the `RpcListener` field, which has a default value of `127.0.0.1:15812`.
You can also specify a custom address using the `--rpc-listener` flag.

```bash
$ fpd start --rpc-listener '127.0.0.1:8088'

time="2023-11-26T16:37:00-05:00" level=info msg="successfully connected to a remote EOTS manager	{"address": "127.0.0.1:15813"}"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting FinalityProviderApp"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting RPC Server"
time="2023-11-26T16:37:00-05:00" level=info msg="RPC server listening	{"address": "127.0.0.1:15812"}"
time="2023-11-26T16:37:00-05:00" level=info msg="Finality Provider Daemon is fully active!"
```

All the available CLI options can be viewed using the `--help` flag. These options
can also be set in the configuration file.

## 5. Create and Register a Finality Provider

A finality provider named `my-finality-provider` can be created in the internal
storage ([bolt db](https://github.com/etcd-io/bbolt))
through the `fpcli create-finality-provider` command.
This finality provider is associated with a BTC public key which
serves as its unique identifier and
a Babylon account to which staking rewards will be directed.
The key name must be the same as the key added in [step](#3-add-key-for-the-consumer-chain).

```bash
$ fpcli create-finality-provider --key-name my-finality-provider \
                --chain-id chain-test --moniker my-name
{
    "babylon_pk_hex": "02face5996b2792114677604ec9dfad4fe66eeace3df92dab834754add5bdd7077",
    "btc_pk_hex": "d0fc4db48643fbb4339dc4bbf15f272411716b0d60f18bdfeb3861544bf5ef63",
    "description": {
        "moniker": "my-name"
    },
    "status": "CREATED"
}
```

The finality provider can be registered with Babylon through
the `register-finality-provider` command.
The output contains the hash of the Babylon
finality provider registration transaction.
Note that if the `key-name` is not specified, the `Key` field of config specified in [step](#3-add-key-for-the-consumer-chain)
will be used.

```bash
$ fpcli register-finality-provider \
                 --btc-pk d0fc4db48643fbb4339dc4bbf15f272411716b0d60f18bdfeb3861544bf5ef63
{
    "tx_hash": "800AE5BBDADE974C5FA5BD44336C7F1A952FAB9F5F9B43F7D4850BA449319BAA"
}
```

To verify that your finality provider has been created,
we can check the finality providers that are managed by the daemon and their status.
These can be listed through the `fpcli list-finality-providers` command.
The `status` field can receive the following values:

- `CREATED`: The finality provider is created but not registered yet
- `REGISTERED`: The finality provider is registered but has not received any active delegations yet
- `ACTIVE`: The finality provider has active delegations and is empowered to send finality signatures
- `INACTIVE`: The finality provider used to be ACTIVE but the voting power is reduced to zero
- `SLASHED`: The finality provider is slashed due to malicious behavior 
 
```bash
$ fpcli list-finality-providers
{
    "finality-providers": [
        ...
        {
            "babylon_pk_hex": "02face5996b2792114677604ec9dfad4fe66eeace3df92dab834754add5bdd7077",
            "btc_pk_hex": "d0fc4db48643fbb4339dc4bbf15f272411716b0d60f18bdfeb3861544bf5ef63",
            "description": {
                "moniker": "my-name"
            },
            "last_vote_height": 1
            "status": "REGISTERED"
        }
    ]
}
```
