## Prerequisites

#### Create a Babylon keyring with funds

The `vald` daemon requires a keyring with loaded funds to pay for the transactions.
These transactions include randomness commits and vote submissions. Follow this
guide [Getting Testnet Tokens](https://docs.babylonchain.io/docs/user-guides/btc-timestamping-testnet/getting-funds)
to create a keyring and request funds.

## Validator daemon (`vald`) configuration

The `valcli` tools serves as a control plane for the Validator Daemon (`vald`).
Below, instructions are provided for configuring the daemons.

The `valcli admin init` command initializes a home directory for the BTC
validator daemon. This directory is created in the default home location or in a
location specified by the `--home` flag.

```bash
$ valcli admin init --home /path/to/vald-home/
```

After initialization, the home directory will have the following structure

```bash
$ ls /path/to/vald-home/
  ├── vald.conf # Vald-specific configuration file.
  ├── logs      # Vald logs
```

If the `--config-file-dir` flag is not specified, then the default home directory
will be used. For different operating systems, those are:

- **MacOS** `~/Library/Application Support/Vald`
- **Linux** `~/.Vald`
- **Windows** `C:\Users\<username>\AppData\Local\Vald`

Below are some important parameters of the `vald.conf` file.

**Note:**

The `Key` parameter in the config below is the name of the key in the keyring to use
for signing transactions. Use the key name you created
in [Create a Babylon keyring with funds](#create-a-babylon-keyring-with-funds)

```bash
# Address of the EOTS Daemon
EOTSManagerAddress = 127.0.0.1:15813

# Babylon specific parameters

# Chain id of the chain (Babylon)
ChainID = chain-test

# Address of the chain's RPC server (Babylon)
RPCAddr = http://localhost:26657

# Address of the chain's GRPC server (Babylon)
GRPCAddr = https://localhost:9090

# Name of the key in the keyring to use for signing transactions
Key = node0

# Type of keyring to use,
# supported backends - (os|file|kwallet|pass|test|memory)
# ref https://docs.cosmos.network/v0.46/run-node/keyring.html#available-backends-for-the-keyring
KeyringBackend = test

# Directory to store validator keys in
KeyDirectory = /Users/<user>/Library/Application Support/Vald/data
```

To change the babylon rpc/grpc address, you can set

```bash
RPCAddr = https://rpc.devnet.babylonchain.io:443
GRPCAddr = https://grpc.devnet.babylonchain.io:443
```

To see the complete list of configuration options, check the `vald.conf` file.
