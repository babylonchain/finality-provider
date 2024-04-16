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
    "key": "Ay+3n0i8B14iaqPzfYdyEUfsRISPHKSP8IteCQJELJVf"
  },
  "btc_pk": "e36171a9efa696aa444c0f02c03afebd675938a0db66f1450fd7479417c44f3c",
  "pop": {
    "babylon_sig": "BOF8XAXuUfQhdAA/WubB8BkCEBsgD+1QufhN4liKGzdA4eZDbh8vjiYnDmJWXQgswjQXovSttZiRy+HAxz/p7w==",
    "btc_sig": "5G18w/zXvQkUSOeB1pSLqzw0JcwyK+t4eoRpVv0zt0kttZCUkkSlB17YwhSYlhCLvNpUke5M2rdYWFi61OqvMw=="
  },
  "master_pub_rand": "xpub661MyMwAqRbcEb5uJdjkpdZNH3NeUinTLbxePMCoVnfT5RBdZjqypSzgHYNhVJr2YiJxb4QzQrdftPoCmZ3qGxo3xCCm2hZvNgH9gSLspHm"
}
```

Where a finality with 5% comission is created.
