# Staking operations

Before proceeding, make sure you have installed the required binaries, configured and
started both EOTS and the BTC validator daemon.

- [Starting the EOTS daemon](./eotsd/eotsd-startup-guide.md)
- [Starting the BTC validator daemon](./vald/vald-startup-guide.md)

The following guide will show how to interact with the daemons to create a BTC
validator and register it to Babylon.

### 1. Creating a BTC validator

A BTC Validator named `my_validator` can be created in the internal
storage ([bolt db](https://github.com/etcd-io/bbolt))
through the `valcli daemon create-validator` command. This validator is associated
with a BTC public key, serving as its unique identifier, and a Babylon account to
which staking rewards will be directed.

```bash
$ valcli daemon create-validator --key-name my-validator --chain-id chain-test 
--passphrase mypassphrase
{
    "btc_pk": "903fab42070622c551b188c983ce05a31febcab300244daf7d752aba2173e786"
}
```

### 2. Registering a validator to Babylon

The BTC validator can be registered with Babylon through the `register-validator`
command. The output contains the hash of the validator registration Babylon
transaction.

```bash
$ valcli daemon register-validator --btc-pk 903fab42070622c551b188c983ce05a31febcab300244daf7d752aba
{
    "tx_hash": "800AE5BBDADE974C5FA5BD44336C7F1A952FAB9F5F9B43F7D4850BA449319BAA"
}
```

### 3. Querying the validators managed by the daemon

The BTC validators that are managed by the daemon can be listed through the
`valcli daemon list-validators` command. The `status` field can receive the following
values:

- `1`: The Validator is active and has received no delegations yet
- `2`: The Validator is active and has staked BTC tokens
- `3`: The Validator is inactive (i.e. had staked BTC tokens in the past but not
  anymore OR has been slashed)
  The `last_committed_height` field is the Babylon height up to which the Validator
  has committed sufficient EOTS randomness

```bash
$ valcli daemon list-validators
{
    "validators": [
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
