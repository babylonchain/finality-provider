## Validator daemon (`vald`) configuration

### Prerequisites

#### 1. Babylond Setup

Download and install the Babylon source code to access the `babylond` tool. For more
information see
the [Babylon installation docs](https://docs.babylonchain.io/docs/user-guides/installation#step-2-build-and-install-babylon-)

```bash
# clone babylon source code
$ git clone git@github.com:babylonchain/babylon.git 

# select a specific version from the official releases page
$ cd babylon
$ git checkout <release-tag>

# build and install the binaries 
# to your $GOPATH/bin directory
$ make install
```

#### 2. Keyring Setup

The `vald` daemon requires a keyring with loaded funds to pay for the transactions.
These transactions include randomness commits and vote submissions. Follow these
steps to set up a keyring using the `babylond` tool:

```bash
$ babylond keys add <key-name> --keyring-backend test
```

It is recommended to use
the [test](https://docs.cosmos.network/v0.46/run-node/keyring.html#the-test-backend)
keyring backend as it does not encrypt the keys on disk.

#### 3. Get funds from the Babylon

Follow this guide
[Getting Testnet Tokens](https://docs.babylonchain.io/docs/user-guides/btc-timestamping-testnet/getting-funds#2-request-funds-from-the-babylon-testnet-faucet)
to request funds from the Babylon testnet faucet to the key created in the previous
step.

### Configuration

The `valcli` tools serve as control plane for the Validator Daemon (`vald`). Below,
instructions are provided for configuring the daemons.

The `valcli admin dump-config` command initializes a home directory for the BTC
validator daemon. This directory is created in the default home location or in a
location specified by the `--config-file-dir` flag.

```bash
$ valcli admin dump-config --config-file-dir /path/to/vald-home/
```

After initialization, the home directory will have the following structure

```bash
$ ls /path/to/vald-home/
  ├── vald.conf # Vald-specific configuration file.
```

If the `--config-file-dir` flag is not specified, then the default home directory
will be used. For different operating systems, those are:

- **MacOS** `~/Library/Application Support/Vald`
- **Linux** `~/.Vald`
- **Windows** `C:\Users\<username>\AppData\Local\Vald`

Below are some important parameters of the `vald.conf` file.

**Note:**

The `Key` parameter in the config below is the name of the key in the keyring to use
for signing transactions. Use the key name you created in
the [Keyring Setup](#keyring-setup)

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

# Type of keyring to use
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
