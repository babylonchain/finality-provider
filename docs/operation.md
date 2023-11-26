# Operations

Before you proceed, ensure that you have installed the necessary binaries and configured the daemons. If you haven't done so yet, please follow these steps:

1. [Installation](installation.md)
2. [Configuration](configuration.md)

## 1. Starting the EOTS Daemon
You can start the EOTS daemon using the following command:

```bash
$ eotsd
```
This will start the rpc server at the default address i.e. `localhost:15813`. You can also specify a custom address using the `--rpclistener` flag.

```bash
$ eotsd --rpclistener 'localhost:8081'
```

All the available cli options can be viewed using the `--help` flag. These options can also be set in the configuration file.

```bash
$ eotsd --help

Usage:
  eotsd [OPTIONS]

Application Options:
      --loglevel=[trace|debug|info|warn|error|fatal] Logging level for all subsystems (default: debug)
      --workdir=                                     The base directory that contains the EOTS manager's data, logs, configuration file, etc. (default:
                                                     /Users/gurjotsingh/Library/Application Support/Eotsd)
      --configfile=                                  Path to configuration file (default: /Users/gurjotsingh/Library/Application
                                                     Support/Eotsd/eotsd.conf)
      --datadir=                                     The directory to store validator's data within (default: /Users/gurjotsingh/Library/Application
                                                     Support/Eotsd/data)
      --logdir=                                      Directory to log output. (default: /Users/gurjotsingh/Library/Application Support/Eotsd/logs)
      --dumpcfg                                      If config file does not exist, create it with current settings
      --key-dir=                                     Directory to store keys in (default: /Users/gurjotsingh/Library/Application Support/Eotsd/data)
      --keyring-type=                                Type of keyring to use (default: file)
      --backend=                                     Possible database to choose as backend (default: bbolt)
      --path=                                        The path that stores the database file (default: bbolt-eots.db)
      --name=                                        The name of the database (default: default)
      --rpclistener=                                 the listener for RPC connections, e.g., localhost:1234 (default: localhost:15813)

Help Options:
  -h, --help                                         Show this help message
```

## 2. Starting the Validator Daemon

You can start the validator daemon using the following command:

```bash
$ vald
```
This will start the rpc server at the default port i.e. `15812`. You can also specify a custom port using 
the `--rpclisten` flag.

```bash
$ vald --rpclisten '8082'
```

All the available cli options can be viewed using the `--help` flag. These options can also be set in the configuration file.

```bash
$ vald --help

Usage:
  vald [OPTIONS]

Application Options:
      --debuglevel=[trace|debug|info|warn|error|fatal] Logging level for all subsystems (default: info)
      --chainname=[babylon]                            the name of the consumer chain (default: babylon)
      --validatorddir=                                 The base directory that contains validator's data, logs, configuration file, etc. (default:
                                                       /Users/gurjotsingh/Library/Application Support/Vald)
      --configfile=                                    Path to configuration file (default: /Users/gurjotsingh/Library/Application
                                                       Support/Vald/vald.conf)
      --datadir=                                       The directory to store validator's data within (default: /Users/gurjotsingh/Library/Application
                                                       Support/Vald/data)
      --logdir=                                        Directory to log output. (default: /Users/gurjotsingh/Library/Application Support/Vald/logs)
      --dumpcfg                                        If config file does not exist, create it with current settings
      --numPubRand=                                    The number of Schnorr public randomness for each commitment (default: 100)
      --numpubrandmax=                                 The upper bound of the number of Schnorr public randomness for each commitment (default: 100)
      --minrandheightgap=                              The minimum gap between the last committed rand height and the current Babylon block height
                                                       (default: 10)
      --statusupdateinterval=                          The interval between each update of validator status (default: 5s)
      --randomnesscommitinterval=                      The interval between each attempt to commit public randomness (default: 5s)
      --submissionretryinterval=                       The interval between each attempt to submit finality signature or public randomness after a
                                                       failure (default: 1s)
      --unbondingsigsubmissioninterval=                The interval between each attempt to check and submit unbonding signature (default: 20s)
      --maxsubmissionretries=                          The maximum number of retries to submit finality signature or public randomness (default: 20)
      --fastsyncinterval=                              The interval between each try of fast sync, which is disabled if the value is 0 (default: 20s)
      --fastsynclimit=                                 The maximum number of blocks to catch up for each fast sync (default: 10)
      --fastsyncgap=                                   The block gap that will trigger the fast sync (default: 6)
      --eotsmanageraddress=                            The address of the remote EOTS manager; Empty if the EOTS manager is running locally (default:
                                                       127.0.0.1:15813)
      --bitcoinnetwork=[regtest|testnet|simnet|signet] Bitcoin network to run on (default: simnet)
      --covenantmode                                   If the program is running in Covenant mode
      --rpclisten=                                     Add an interface/port/socket to listen for RPC connections

chainpollerconfig:
      --chainpollerconfig.buffersize=                  The maximum number of Babylon blocks that can be stored in the buffer (default: 1000)
      --chainpollerconfig.pollinterval=                The interval between each polling of Babylon blocks (default: 5s)

databaseconfig:
      --databaseconfig.backend=                        Possible database to choose as backend (default: bbolt)
      --databaseconfig.path=                           The path that stores the database file (default: bbolt.db)
      --databaseconfig.name=                           The name of the database (default: default)

eotsmanagerconfig:
      --eotsmanagerconfig.dbbackend=                   Possible database to choose as backend (default: bbolt)
      --eotsmanagerconfig.dbpath=                      The path that stores the database file (default: bbolt-eots.db)
      --eotsmanagerconfig.dbname=                      The name of the database (default: eots-default)

babylon:
      --babylon.key=                                   name of the key to sign transactions with (default: node0)
      --babylon.chain-id=                              chain id of the chain to connect to (default: chain-test)
      --babylon.rpc-address=                           address of the rpc server to connect to (default: http://localhost:26657)
      --babylon.grpc-address=                          address of the grpc server to connect to (default: https://localhost:9090)
      --babylon.acc-prefix=                            account prefix to use for addresses (default: bbn)
      --babylon.keyring-type=                          type of keyring to use (default: test)
      --babylon.gas-adjustment=                        adjustment factor when using gas estimation (default: 1.2)
      --babylon.gas-prices=                            comma separated minimum gas prices to accept for transactions (default: 0.01ubbn)
      --babylon.key-dir=                               directory to store keys in (default: /Users/gurjotsingh/Library/Application Support/Vald/data)
      --babylon.debug                                  flag to print debug output
      --babylon.timeout=                               client timeout when doing queries (default: 20s)
      --babylon.block-timeout=                         block timeout when waiting for block events (default: 1m0s)
      --babylon.output-format=                         default output when printint responses (default: json)
      --babylon.sign-mode=                             sign mode to use (default: direct)

validator:
      --validator.staticchainscanningstartheight=      The static height from which we start polling the chain (default: 1)
      --validator.autochainscanningmode                Automatically discover the height from which to start polling the chain

covenant:
      --covenant.covenantkeyname=                      The key name of the Covenant if the program is running in Covenant mode (default: covenant-key)
      --covenant.queryinterval=                        The interval between each query for pending BTC delegations (default: 15s)
      --covenant.delegationlimit=                      The maximum number of delegations that the Covenant processes each time (default: 100)
      --covenant.slashingaddress=                      The slashing address that the slashed fund is sent to

Help Options:
  -h, --help                                           Show this help message
```

**Note**: It is recommended to run the `eotsd` daemon on a separate machine or network segment to enhance security.
This helps isolate the key management functionality and reduces the potential attack surface. You can edit the
`EOTSManagerAddress` in  `vald.conf`  to reference the address of the machine where `eotsd` is running.

## 3. Interacting with daemons

### Creating a validator

Create a BTC Validator named `my_validator` in the internal storage ([bolt db](https://github.com/etcd-io/bbolt))
This Validator holds a BTC public key (where the staked tokens will be sent to) and a Babylon account
(where the Babylon reward tokens will be sent to).

```bash
$ valcli daemon create-validator --key-name my-validator --chain-id chain-test

{
    "btc_pk": "903fab42070622c551b188c983ce05a31febcab300244daf7d752aba2173e786"
}
```


### Registering a validator to Babylon

Register the Validator with Babylon. Now, the Validator is ready to receive
delegations. The output contains the hash of the validator registration
Babylon transaction.

```bash
$ valcli daemon register-validator --btc-pk 903fab42070622c551b188c983ce05a31febcab300244daf7d752aba

{
    "tx_hash": "800AE5BBDADE974C5FA5BD44336C7F1A952FAB9F5F9B43F7D4850BA449319BAA"
}
```

### Querying the validators managed by the daemon

List all the BTC Validators managed by the BTC Validator daemon. The `status`
field can receive the following values:
- `1`: The Validator is active and has received no delegations yet
- `2`: The Validator is active and has staked BTC tokens
- `3`: The Validator is inactive (i.e. had staked BTC tokens in the past but
not anymore OR has been slashed)
The `last_committed_height` field is the Babylon height up to which the
Validator has committed sufficient EOTS randomness

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
