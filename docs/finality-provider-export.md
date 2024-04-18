# Finality Provider Export

Finality providers are responsible for voting at a finality round on top of CometBFT that is powered by BTC stake.
Similar to any native PoS validator, finality providers can receive voting power delegations from BTC stakers, and
through their voting power provide economic security to the underlying PoS protocol.
In this document,
we explore how someone can create a finality provider and export details about it in JSON format.

## Prerequisite

### Install binaries

[Follow these instructions](../README.md#2-installation) to install the binaries.

- `eotsd`
- `fpd`
- `fpcli`

### Setup EOTS

The eots is responsible for managing EOTS keys, producing EOTS randomness,
and using them to produce EOTS signatures.
Each finality provider needs to provide public randomness.
So, to export one finality provider it is needed to register a new key.

To initialize the eots work directory, run `eotsd init`. It creates in the default
home location, unless `--home` flag is specified.

```shell
$ eotsd init --home ./export-fp/eots
$ ls ./export-fp/eots
data/  eotsd.conf  logs/
```

> Creates the config file and two directories for logs and data.

After the proper configuration, start the etos daemon with the command `eotsd start`,
The home folder should point to the work directory previously created with `eotsd init`.

```shell
$ eotsd start --home ./export-fp/eots
2024-04-17T16:52:10.553893Z     info    Metrics server is starting      {"addr": "127.0.0.1:2113"}
2024-04-17T16:52:10.554021Z     info    RPC server listening    {"address": "127.0.0.1:12582"}
2024-04-17T16:52:10.554061Z     info    EOTS Manager Daemon is fully active!
```

> Starts the eots process that can be turned down after the finality provider is exported
> (run all commands of this file).

### Setup fpd

The fpd is reposible for monitoring for new Babylon blocks, committing public
randomness for the blocks it intends to provide finality signatures for,
and submitting finality signatures. To just create one finality provider and
export his information, there is no need to connect to babylon chain, just
initialize the working directory for the finality provider config.
To initialize the fpd work directory, run `fpd init`. It creates
in the default home location, unless `--home` flag is specified.

```shell
$ fpd init --home ./export-fp/fpd
$ ls ./export-fp/eots
fpd.conf  logs/
```

> Creates the config file and one directory for logs.

The finality provider need a private and public key on the consumer chain,
in this case `babylon`, for creating a key for the finality provider run
`fpd keys add --key-name [my-name]`. For this command, several flags are available:

- `--key-name` mandatory as it identifies the name for the key to be created.
- `--chain-id` mandatory as it specifies the chain ID to be used for context
creation of the key.
- `--home` flag needs to be the same as used in the `fpd init` command,
to load the configuration.
- `--keyring-backend` specifies the keyring options, any of `[file, os, kwallet, test, pass, memory]`
are available, by default `test` is used.
- `--passphrase` sets a password to encrypt the keys, this is a optional flag with no defaults.
- `--hd-path` defines the hd path to use for derivation of the private key.

```shell
$ fpd keys add --home ./export-fp/fpd --chain-id babylon-1 --key-name finality-provider
New key for the consumer chain is created (mnemonic should be kept in a safe place for recovery):
{
  "name": "finality-provider",
  "address": "bbn1d54vq462hyh0jj07hmxrn2p59p4axn5mwlzqeu",
  "mnemonic": "bad mnemonic actress issue error swap sphere excuse anxiety machine meat immense rebuild adapt color push polar decorate poverty material skin wear battle zebra"
}
```

> Creates one key pair identified by the key name `finality-provider`

## Creation & Export of Finality provider

Finally, after setup of `eots` and `fdp`, to create and export the finality
provider, run `fpcli export-finality-provider`.
This command connects with the eots manager daemon process, creates one key and
produces the finality provider export information.

> Obs.: This command does not send a transaction to babylon chain `babylond tx btcstaking create-finality-provider`.

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
$ fpcli export-finality-provider --home ./export-fp/fpd --key-name finality-provider \
--chain-id babylon-1 --commission 0.05 --moniker my-fp-nickname --identity anyIdentity \
--website www.my-public-available-website.com --security-contact your.email@gmail.com \
--details 'other overall info'
```

The expected result is a finality with 5% comission exported:

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
    "key": "AzHiB4T9Za1H7pn9NB95UhUJLQ0vOpUAx82jKUtdkyka"
  },
  "btc_pk": "5affe98cd7b180e2822d4d25fd8fab2dafd6b31a9441b3f6c593022fc4d30e5a",
  "pop": {
    "babylon_sig": "waCl0LFEs8m3vSE6cZoDNb4qRMheZtrGgpvNiZptmb4xueLfvoP8y/b2MqOlBiBSsmfypYni468eICGsO0ITmA==",
    "btc_sig": "KxPlo28i7H9IH3fJAAe/ZsuOYdUkGcEw+nnv1BxgakFycW85xag69js6Q5zmvuO++MFh0JbbZq+lTjneE9tosQ=="
  },
  "master_pub_rand": "xpub661MyMwAqRbcG23M9EWAJw71GYxJWfnU47bqCw9gjnALYB1vPQkG6cnkkxyU1LriBi5JXCZb8XK2r454NSnPRrdVxaZNJs9bVKdj4ff3NkC",
  "fp_sig_hex": "dad8205a2686a38e01bc1d2dd20366981fcd381d0fe2d330ddc415dcb8f507e6407672aac39d121f2d3683ed8ad7bd53241047a1c28d7b163db7c4c5256bc1ba"
}
```

> The eots manager (`eotsd start`) can be turned down.
