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
fpd init --home /path/to/fpd/home/
```

After initialization, the home directory will have the following structure

```bash
ls /path/to/fpd/home/
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

# Type of keyring to use
KeyringBackend = test

# Directory where keys will be retrieved from and stored
KeyDirectory = /path/to/fpd/home
```

To see the complete list of configuration options, check the `fpd.conf` file.

**Additional Notes:**

1. We strongly recommend that EOTS randomness commitments are limited to 500
   blocks (default value: 100 blocks)

   ```bash
   ; The number of Schnorr public randomness for each commitment
   NumPubRand = 100

   ; The upper bound of the number of Schnorr public randomness for each commitment
   NumPubRandMax = 100
   ```

2. If you encounter any gas-related errors while performing staking operations,
   consider adjusting the `GasAdjustment` and `GasPrices` parameters. For example,
   you can set:

   ```bash
   GasAdjustment = 1.5
   GasPrices = 0.01ubbn
   ```

## 3. Add key for the consumer chain

The finality provider daemon requires the existence of a keyring that contains
an account with Babylon token funds to pay for transactions.
This key will be also used to pay for fees of transactions to the consumer chain.

Use the following command to add the key:

```bash
fpd keys add --key-name my-finality-provider --chain-id chain-test
```

After executing the above command, the key name will be saved in the config file
created in [step](#2-configuration).

## 4. Starting the Finality Provider Daemon

You can start the finality provider daemon using the following command:

```bash
fpd start --home /path/to/fpd/home
```

This will start the RPC server at the address specified in the configuration under
the `RpcListener` field, which has a default value of `127.0.0.1:15812`.
You can also specify a custom address using the `--rpc-listener` flag.

This will also start all the registered finality provider instances added in [step](#5-create-and-register-a-finality-provider).
To start the daemon with a specific finality provider instance, use the `--fp-btc-pk`
flag followed by the hex string of the BTC public key of the finality provider (`btc_pk_hex`)
obtained in [step](#5-create-and-register-a-finality-provider).

```bash
fpd start --rpc-listener '127.0.0.1:8088'

time="2023-11-26T16:37:00-05:00" level=info msg="successfully connected to a remote EOTS manager	{"address": "127.0.0.1:15813"}"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting FinalityProviderApp"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting RPC Server"
time="2023-11-26T16:37:00-05:00" level=info msg="RPC server listening	{"address": "127.0.0.1:15812"}"
time="2023-11-26T16:37:00-05:00" level=info msg="Finality Provider Daemon is fully active!"
```

All the available CLI options can be viewed using the `--help` flag. These options
can also be set in the configuration file.

## 5. Create and Register a Finality Provider

We create a finality provider instance through the `fpcli create-finality-provider` or `fpcli cfp` command.
The created instance is associated with a BTC public key which
serves as its unique identifier and
a Babylon account to which staking rewards will be directed.
Note that if the `--key-name` flag is not specified, the `Key` field of config specified in [step](#3-add-key-for-the-consumer-chain)
will be used.

```bash
fpcli create-finality-provider --key-name my-finality-provider \
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

We register a created finality provider in Babylon through
the `fpcli register-finality-provider` or `fpcli rfp` command.
The output contains the hash of the Babylon finality provider registration transaction.

```bash
fpcli register-finality-provider \
                 --btc-pk d0fc4db48643fbb4339dc4bbf15f272411716b0d60f18bdfeb3861544bf5ef63
{
    "tx_hash": "800AE5BBDADE974C5FA5BD44336C7F1A952FAB9F5F9B43F7D4850BA449319BAA"
}
```

A finality provider instance will be initiated and start running right after
the finality provider is successfully registered in Babylon.

We can view the status of all the running finality providers through
the `fpcli list-finality-providers` or `fpcli ls` command.
The `status` field can receive the following values:

- `CREATED`: The finality provider is created but not registered yet
- `REGISTERED`: The finality provider is registered but has not received any active delegations yet
- `ACTIVE`: The finality provider has active delegations and is empowered to send finality signatures
- `INACTIVE`: The finality provider used to be ACTIVE but the voting power is reduced to zero
- `SLASHED`: The finality provider is slashed due to malicious behavior
 
```bash
fpcli list-finality-providers
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
