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

###  Setting up EOTS daemon

```bash
./build/eotsd admin dump-config
```
Ensure you have created the `Eotsd` directory in the config directory. The default config directories are:

    MacOS ~/Library/Application Support/Eotsd 
    Linux ~/.Eotsd
    Windows C:\Users\<username>\AppData\Local\Etosd

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

### Setting up Validator daemon

```bash
./build/vald admin dump-config
```
Ensure you have created the `Vald` directory in the config directory. The default config directories are:

    MacOS ~/Library/Application Support/Vald 
    Linux ~/.Vald
    Windows C:\Users\<username>\AppData\Local\Vald

The **Validator Daemon** is responsible for
1.  **Finality Signatures:**
    -   Sends the finality signature to the consumer chain (Babylon) for each registered validator and for each block.
2.  **Randomness Commitment:**
    -   Ensures the inclusion of public randomness in each block it processes.

## 5. Running Daemons

Run in different terminals

    ./build/vald
    ./build/eotsd

## 6. Creating a Validator and Registering to Babylon

### Create a validator in internal db

    ./build/valcli daemon create-validator --key-name uniqueIdentifier --chain-id chain-test
    
    # Response 
    {
        "btc_pk": "ab6f16c7f532570bade5d05b00ead6d397936029b99cd68f1c66e5b007621134"
    }


### Register the validator to Babylon
    ./build/valcli daemon register-validator --btc-pk ab6f16c7f532570bade5d05b00ead6d397936029b99cd68f1c66e5b007621134
