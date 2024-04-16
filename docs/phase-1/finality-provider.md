# Finality Provider

Finality providers are responsible for voting at a finality round on top of CometBFT. Similar to any native PoS validator, a finality provider can receive voting power delegations from BTC stakers, and can earn commission from the staking rewards denominated in Babylon tokens.

## Requirements

- [eotsd](https://github.com/babylonchain/finality-provider)
- [fpd](https://github.com/babylonchain/finality-provider)

## Creation & Export of Finality provider

- Start by creating the finality provider config, this config is needed to define the `[dbconfig]` where it is set the path and database configurations

```shell
$~ fpd init --home ./phase-1/fpd
```

- The finality provider need a private and public key on the consumer chain, in this case `babylon`, for creating a key run the following.:

```shell
$~ fpd keys add finality-provider --home ./phase-1/fpd --chain-id babylon-1 --key-name finality-provider
```

- Each finality provider needs to provide public randomness, for generating and managing such a thing the eots is used. Start by initiating the config file of eots with:

```shell
$~ eotsd init --home ./phase-1/eots
```

- In another terminal, start the eots manager daemon.

```shell
$~ eotsd start --home ./phase-1/eots
```

- On this phase no babylon or consumer chain is not running, so it is not possible to register the finality provider onchain, but for BTC delegations to choose your finality provider
for staking their BTC, there is a need to get information about you, so the solution is to export the finality provider information with the following CLI:

```shell
$~ fpcli p1-export-finality-provider --home ./phase-1/fpd --key-name finality-provider --chain-id babylon-1 --commission 0.05 --moniker my-fp-nickname --identity anyIdentity --website www.my-public-available-website.com --security-contact email-for-questions@gmail.com --details 'other overall info'
```

The expected result is:

```json
{
  "description": {
    "moniker": "my-fp-nickname",
    "identity": "anyIdentity",
    "website": "www.my-public-available-website.com",
    "security_contact": "email-for-questions@gmail.com",
    "details": "other overall info"
  },
  "commission": "0.050000000000000000",
  "babylon_pk": {
    "key": "A6swHKxNoqAP9BLnqiY6VwNSasNiC1u/1TylRUZZzepA"
  },
  "btc_pk": "ff06bfc18a3e5d44836a8d310be05a8648f8e4ebedeb88420bf5a2255e5f9e29",
  "pop": {
    "babylon_sig": "c9g/L8hlVGTf6/4Zk++P+JzX6qSOXRjWO948NrsQpy1WWRLq82/pqv0HUo0nZHDbPjFmGtITZMDMlIQkCGtGKg==",
    "btc_sig": "L6XvFY+nJw57VWafo5rAmjbGt5dOFmGkbsBQKGVd8Z8kVJtQhDf1qOAMCBUqJB6jmu37vmZgzTXgJL5BuzG8FQ=="
  },
  "master_pub_rand": "xpub661MyMwAqRbcFRjHEMroeyKJhPp76kdAHMh5D1bZSNEeaJHuKK74BucAGESCXjykzqFUZtZQFDxqg8cLFk6iuVYe26oAPFoPXBYnTAKGnEf",
  "fp_sig_hex": "87ca116fbd2c919060dc8dfbd429b424f3c25bb979a344c6e6c2b04b639ec7d33b84ed047413d3968dfa11a030936d90e2b09687f1c9c3add17293cc9122dde3"
}
```

Where a finality with 5% comission is created.
