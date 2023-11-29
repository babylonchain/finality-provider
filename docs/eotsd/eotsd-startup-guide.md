## Prerequisites

1. **Install Binaries:**
   Follow the instructions in
   the [installation section](../../README.md#2-installation) to install the required
   binaries.

2. **EOTS Daemon Configuration:**
   Follow the instructions in the [EOTS Daemon Configuration](eotsd-config.md) guide
   to configure the EOTS daemon.

## Starting the EOTS Daemon

You can start the EOTS daemon using the following command:

```bash
$ eotsd
```

This will start the rpc server at the address specified in the configuration under
the `RpcListener` field. It can also be overridden with custom address using
the `--rpclistener` flag.

```bash
$ eotsd --rpclistener 'localhost:8081'

time="2023-11-26T16:35:04-05:00" level=info msg="RPC server listening on 127.0.0.1:8081"
time="2023-11-26T16:35:04-05:00" level=info msg="EOTS Manager Daemon is fully active!"
```

All the available cli options can be viewed using the `--help` flag. These options
can also be set in the configuration file.

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

**Note**: It is recommended to run the `eotsd` daemon on a separate machine or
network segment to enhance security. This helps isolate the key management
functionality and reduces the potential attack surface. You can edit the
`EOTSManagerAddress` in  `vald.conf`  to reference the address of the machine
where `eotsd` is running.
