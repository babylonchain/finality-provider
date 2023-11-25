# BTC-Validator

BTC-Validator is a stand-alone program that creates and manages all BTC validators' keys and act on behalf of them. Follow the steps below to download, build, and set up the validator and EOTS daemons.

## 1. Downloading the Code

```bash
git clone git@github.com:babylonchain/btc-validator.git
cd btc-validator
git checkout <release-tag>
```

## 2. Building the binary

```bash
make build
```

## 3. Configuring Validator and EOTS Daemon

To operate the BTC-Validator program, you need to run two essential daemons: **EOTS Daemon (`eotsd`)** and **Validator Daemon (`vald`)**.

###  Setting up EOTS daemon

The **EOTS Daemon** is responsible for

1.  **Key Management:**
    -   Generates Schnorr key pairs for the validator using the BIP-340 standard.
    -   Persists generated key pairs in storage.
    -   Retrieves information about the validator's key record.
2.  **Randomness Generation:**
    -   Generates lists of Schnorr randomness pairs based on the EOTS key, chainID, and block height.
    -   The randomness is deterministically generated and tied to specific parameters.
3.  **Signature Generation:**
    -   Signs EOTS using the private key of the validator and corresponding secret randomness for a given chain at a specified height.
    -   Signs Schnorr signatures using the private key of the validator.

#### Dump the default config

```bash
./build/eotsd admin dump-config
```

Ensure you have created the `Eotsd` directory in the config directory. The default config directories are:

    MacOS ~/Library/Application Support/Eotsd 
    Linux ~/.Eotsd
    Windows C:\Users\<username>\AppData\Local\Eotsd

or

You can also specify custom path to store the config
```bash
./build/vald admin dump-config --config-file-dir /path/to/your/config/
```

After dump the config directory has the following structure
```bash
ls ~/Library/Application Support/Eotsd
    └── eotsd.conf                    # Eots-specific configuration file.
```

### Setting up Validator daemon

The **Validator Daemon** is responsible for
1.  **Finality Signatures:**
    -   Sends the finality signature to the consumer chain (Babylon) for each registered validator and for each block.
2.  **Randomness Commitment:**
    -   Ensures the inclusion of public randomness in each block it processes.

#### Dump the default config

```bash
./build/vald admin dump-config
```
Ensure you have created the `Vald` directory in the config directory. The default config directories are:

    MacOS ~/Library/Application Support/Vald 
    Linux ~/.Vald
    Windows C:\Users\<username>\AppData\Local\Vald
or

You can also specify custom path to store the config

```bash
./build/vald admin dump-config --config-file-dir /path/to/your/config/
```

After dump the config directory has following structure
```bash
ls /path/to/your/config/Vald
    ├── data                          # Contains Vald-specific data.
    ├── logs                          # Contains logs generated by Vald.
    └── vald.conf                     # Vald-specific configuration file.
```

### Updating Some Default Settings
If you want to change any field values in configuration files (for ex: vald.conf) you can use  `sed` commands to do 
that. Few examples are listed here.
    
```bash
# to change the babylon rpc address
sed -i '' 's/RPCAddr = http:\/\/localhost:26657/RPCAddr = https:\/\/rpc.devnet.babylonchain.io/' /path/to/your/vald.conf

# to change the signing key
sed -i '' 's/Key = node0/Key = new-key-name/' /path/to/your/vald.conf
```

## 5. Running Daemons

### Running EOTS Daemon (`eotsd`)

```bash
    ./build/eotsd
```

**Note**: It is recommended to run the `eotsd` daemon on a separate machine or network segment to enhance security. 
This helps isolate the key management functionality and reduces the potential attack surface. You can edit the 
`EOTSManagerAddress` in  `vald.conf`  to reference the address of the remote machine where `eotsd` is running.

### Running Validator Daemon (`vald`)

```bash
    ./build/vald
```

## 6. Creating a Validator and Registering to Babylon

### Create a validator in internal db

```bash
./build/valcli daemon create-validator --key-name unique-id --chain-id chain-test

# response 
{
    "btc_pk": "ab6f16c7f532570bade5d05b00ead6d397936029b99cd68f1c66e5b007621134"
}
```

### Register the validator to Babylon

```bash
./build/valcli daemon register-validator --btc-pk ab6f16c7f532570bade5d05b00ead6d397936029b99cd68f1c66e5b007621134
```

### Confirm the validator is registered to Babylon

```bash
./build/valcli daemon validator-info --btc-pk ab6f16c7f532570bade5d05b00ead6d397936029b99cd68f1c66e5b007621134
```
