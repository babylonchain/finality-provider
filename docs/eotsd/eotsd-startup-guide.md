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
$ eotsd --home /path/to/eotsd/home
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

**Note**: It is recommended to run the `eotsd` daemon on a separate machine or
network segment to enhance security. This helps isolate the key management
functionality and reduces the potential attack surface. You can edit the
`EOTSManagerAddress` in  `vald.conf`  to reference the address of the machine
where `eotsd` is running.
