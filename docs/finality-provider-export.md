# Finality Provider Export

Finality providers are responsible for voting at a finality round on top of
CometBFT that is powered by BTC stake. Similar to any native PoS validator,
finality providers can receive voting power delegations from BTC stakers, and
through their voting power provide economic security to the underlying PoS protocol.
In this document, we explore how someone can create a finality provider and
export details about it in JSON format.

## Prerequisite

### Install binaries

[Follow these instructions](../README.md#2-installation) to install the binaries.

- `eotsd`: EOTS manager daemon to create and manage EOTS keys for each finality
provider and provide finality signatures.
- `fpd`: Finality provider daemon to host the finality provider instance.
- `fpcli`: User interface of the finality provider that requires `eotsd` or `fpd`
is running.

### Setup EOTS manager daemon

We need the EOTS manager daemon (`eotsd`) to create and manage EOTS keys for each
finality provider. To initialize the `eotsd` work directory, run `eotsd init`.
It creates the default home location, unless `--home` flag is specified.

```shell
$ eotsd init --home ./export-fp/eots
$ ls ./export-fp/eots
data/  eotsd.conf  logs/
```

> Creates the config file and two directories for logs and data.

After the setup of `eots` config, update the `DBPath` config property of
`[dbconfig]` in `./export-fp/eots/eotsd.conf` to the data directory created inside
`finality-provider/export-fp/eots/data`. To store all the data in one place for easy backup.

![image](https://github.com/babylonchain/finality-provider/assets/17556614/5a510281-7fa2-4d9c-a3e6-13e8263b4a71)

After the proper configuration, start the eots daemon with the command `eotsd start`.
The home folder should point to the work directory previously created with `eotsd init`.

```shell
$ eotsd start --home ./export-fp/eots
2024-04-17T16:52:10.553893Z     info    Metrics server is starting      {"addr": "127.0.0.1:2113"}
2024-04-17T16:52:10.554021Z     info    RPC server listening    {"address": "127.0.0.1:12582"}
2024-04-17T16:52:10.554061Z     info    EOTS Manager Daemon is fully active!
```

> Starts the eots process that can be turned down after the finality provider
> is exported (run all commands of this file).

### Setup Finality Provider Daemon

The finality provider daemon (fpd) is responsible for polling consumer chain blocks
and providing finality signatures if it is in the active finality provider set.
To initialize the `fpd` work directory, run `fpd init`, which creates the
default home location, unless the `--home` flag is specified.

```shell
$ fpd init --home ./export-fp/fpd
$ ls ./export-fp/eots
fpd.conf  logs/
```

After the setup of `fpd` config, update the `ChainID` config property of `[babylon]`
to the proper chain ID to be used in the `./export-fp/fpd/fpd.conf` file.

![image](https://github.com/babylonchain/finality-provider/assets/17556614/be079679-eb44-4bc8-877e-1bf39dbcd506)

> Creates the config file and one directory for logs.

To connect the finality provider to a specific consumer chain
(`Babylon chain` in this case), we need to generate a key pair for it.
Run `fpd keys add --key-name [my-name]` to create the key pair.
Some available flags:

- `--key-name` mandatory as it identifies the name for the key to be created.
- `--chain-id` mandatory as it specifies the chain ID to be used for context
creation of the key.
- `--home` flag needs to be the same as used in the `fpd init` command,
to load the configuration.
- `--keyring-backend` specifies the keyring options, any of `[file, os, kwallet, test, pass, memory]`
are available, by default `test` is used.
- `--passphrase` sets a password to encrypt the keys, this is a optional flag
with no defaults.
- `--hd-path` defines the hd path to use for derivation of the private key.

```shell
$ fpd keys add --home ./export-fp/fpd --chain-id babylon-1 --key-name finality-provider --keyring-backend file
Enter keyring passphrase (attempt 1/3): ...
Re-enter keyring passphrase: ...
New key for the consumer chain is created (mnemonic should be kept in a safe place for recovery):
{
  "name": "finality-provider",
  "address": "bbn1d54vq462hyh0jj07hmxrn2p59p4axn5mwlzqeu",
  "mnemonic": "bad mnemonic actress issue error swap sphere excuse anxiety machine meat immense rebuild adapt color push polar decorate poverty material skin wear battle zebra"
}
```

**⚠ Store safely the mnemonic and generated keys.**

> Creates one key pair identified by the key name `finality-provider`.
> The added key will be used to create the proof-of-possession (pop)
> of the finality provider.
> For production enviroments, make sure to select a proper
[backend keyring](https://docs.cosmos.network/v0.45/run-node/keyring.html#available-backends-for-the-keyring)
, one of `[os, file, pass, kwallet]`.

After the setup of the configuration and key to be used, start the `fpd` daemon
with the command `fpd start` with no chain backend `--no-chain-backend`.
No chain backend is needed to create and export the finality provider information.
The `fpd` daemon only needs to connect to the backend blockchain to consume blocks
and verify if the finality provider is active to provide finality signatures.
The home folder should point to the work directory previously created with `fpd init`

```shell
$ fpd start --home ./export-fp/fpd --no-chain-backend
2024-04-22T10:15:42.548999Z     info    successfully connected to a remote EOTS manager     {"address": "127.0.0.1:12582"}
2024-04-22T10:15:42.577365Z     info    Starting FinalityProviderApp
2024-04-22T10:15:42.577481Z     info    starting metrics update loop{"interval seconds": 0.1}
2024-04-22T10:15:42.577589Z     info    RPC server listening    {"address": "127.0.0.1:12581"}
2024-04-22T10:15:42.577625Z     info    Finality Provider Daemon is fully active!
2024-04-22T10:15:42.577610Z     info    Metrics server is starting {"addr": "127.0.0.1:2112"}
```

## Create a Finality Provider

After the setup and start of `eots` and `fdp`, to interact with the `fpd`
daemon, you can use the `fpcli` utility. To create the finality provider,
run `fpcli create-finality-provider`.

> Obs.: This command does not send a transaction to babylon chain.

This command also has several flag options.:

- `--key-name` mandatory as it identifies the name for the key to use on fpd creation.
- `--chain-id` mandatory as it specifies the chain ID to be used in the context.
- `--home` flag needs to be the same as used in the `fpd init` command,
to load the configuration.
- `--passphrase` the password used to encrypt the key.
- `--hd-path` the hd derivation path of the private key.
- `--comission` the commission charged from btc stakers rewards, default is `0.05` 5%.
- Multiple flags are used for description and identification of the finality provider
  - `--moniker` nickname of the finality provider.
  - `--identity` optional identity signature (ex. UPort or Keybase).
  - `--website` optional website link.
  - `--security-contact` optional email for security contact.
  - `--details` any other optional detail information.

```shell
$ fpcli create-finality-provider --home ./export-fp/fpd --key-name finality-provider \
--chain-id babylon-1 --commission 0.05 --moniker my-fp-nickname --identity anyIdentity \
--website www.my-public-available-website.com --security-contact your.email@gmail.com \
--details 'other overall info'

{
  "chain_pk_hex": "02d3c46a006a55050bea8834a87f87e38a457fda7759c1c0bdafb30b6fbbe17f29",
  "btc_pk_hex": "25d13990ce6175dc5b5901cdaceb07e337b9b2a6aa39d0f6ae0ad75738dff7c1",
  "description": {
    "moniker": "my-fp-nickname",
    "identity": "anyIdentity",
    "website": "www.my-public-available-website.com",
    "security_contact": "your.email@gmail.com",
    "details": "other overall info"
  },
  "commission": "0.050000000000000000",
  "registered_epoch": 18446744073709551615,
  "master_pub_rand": "xpub661MyMwAqRbcFLhUq9uPM7GncSytVZvoNg4w7LLx1Y74GeeAZerkpV1amvGBTcw4ECmrwFsTNMNf1LFBKkA2pmd8aJ5Jmp8uKD5xgVSezBq",
  "status": "CREATED",
  "pop": {
    "chain_sig": "sAg34vImQTFVlZYsziw9PCCKDuRyZv38V2MX8Ij9fQhyOdpxCUZ1VEgpSlwV/dbnpDs1UOez8Ni9EcbADkmnBA==",
    "btc_sig": "sHLpEHVTyTp9K55oeHxnPlkV4unc/r1obqzKn5S1gq95oXA3AgL1jyCzd/mGb23RfKbEyABjYUdcIBtZ02l5jg=="
  }
}
```

> store the `btc_pk_hex` value to be used in the export command.

## Export a Finality Provider

Finally, after the creation of the finality provider, to export the finality
provider, run `fpcli export-finality-provider`. This command connects with the
`fpd` daemon to load the finality provider previously created using the flag `--btc-pk`.

This command also has several flag options.:

- `--btc-pk` the hex string of the BTC public key.
- `--signed` signs the finality provider as a proof of a untempered exported data.
- `--key-name` mandatory as it identifies the name for the key to use on fpd creation.
- `--home` flag needs to be the same as used in the `fpd init` command,
to load the configuration.
- `--passphrase` the password used to encrypt the key.
- `--hd-path` the hd derivation path of the private key.

```shell
$ fpcli export-finality-provider --btc-pk 25d13990ce6175dc5b5901cdaceb07e337b9b2a6aa39d0f6ae0ad75738dff7c1 \
--home ./export-fp/fpd --key-name finality-provider \
--signed
```

The expected result is a finality with signature exported:

```json
{
  "description": {
    "moniker": "my-fp-nickname",
    "identity": "anyIdentity",
    "website": "www.my-public-available-website.com",
    "security_contact": "your.email@gmail.com",
    "details": "other overall info"
  },
  "commission": "0.050000000000000000",
  "babylon_pk": {
    "key": "AtPEagBqVQUL6og0qH+H44pFf9p3WcHAva+zC2+74X8p"
  },
  "btc_pk": "25d13990ce6175dc5b5901cdaceb07e337b9b2a6aa39d0f6ae0ad75738dff7c1",
  "pop": {
    "babylon_sig": "sAg34vImQTFVlZYsziw9PCCKDuRyZv38V2MX8Ij9fQhyOdpxCUZ1VEgpSlwV/dbnpDs1UOez8Ni9EcbADkmnBA==",
    "btc_sig": "sHLpEHVTyTp9K55oeHxnPlkV4unc/r1obqzKn5S1gq95oXA3AgL1jyCzd/mGb23RfKbEyABjYUdcIBtZ02l5jg=="
  },
  "master_pub_rand": "xpub661MyMwAqRbcFLhUq9uPM7GncSytVZvoNg4w7LLx1Y74GeeAZerkpV1amvGBTcw4ECmrwFsTNMNf1LFBKkA2pmd8aJ5Jmp8uKD5xgVSezBq",
  "fp_sig_hex": "8ded8158bf65d492c5c6d1ff61c04a2176da9c55ea92dcce5638d11a177b999732a094db186964ab1b73c6a69aaa664672a36620dedb9da41c05e88ad981edda"
}
```

> The eots manager (`eotsd start`) can be turned down.
> The fpd daemon (`fpd start`) can be turned down.
> Keep the database folders of `eots` and `fpd` securely stored as it contains
information that will be used latter for running the finality provider.

## Security considerations

All the generated files and keyring must be stored in a secure place for latter usage.
If any keys are lost, BTC or backend chain the finality provider will not be able
to start the chain with BTC delegations prior genesis block and all the BTC locked
in the finality provider that lost his keys will not be providing security
and by consequent, not earn any rewards.

The recommendation is to safely store the entire `./export-fp` directory
that compress occupies roughly 5kb. To compress, run the following command:

```shell
tar czf backup-exported-fpd.tar.gz ./export-fp
```

⚠ Keep the compressed directory stored securely.
