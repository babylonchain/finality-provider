## Prerequisites

1. **Install Binaries:**
   Follow the instructions in
   the [installation section](../../README.md#2-installation) to install the required
   binaries.

2. **Validator Daemon Configuration:**
   Follow the instructions in the [Validator Daemon Configuration](vald-config.md)
   guide to configure the validator daemon.

## Starting the Validator Daemon

You can start the validator daemon using the following command:

```bash
$ vald --home /path/to/vald/home
```

This will start the RPC server at the address specified in the configuration under
the `RawRPCListeners` field. A custom address can also be specified using
the `--rpclisten` flag.

```bash
$ vald --rpclisten 'localhost:8082'

time="2023-11-26T16:37:00-05:00" level=info msg="successfully connected to a remote EOTS manager at 127.0.0.1:8081"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting ValidatorApp"
time="2023-11-26T16:37:00-05:00" level=info msg="Version: 0.2.2-alpha commit=, build=production, logging=default, debuglevel=info"
time="2023-11-26T16:37:00-05:00" level=info msg="Starting RPC Server"
time="2023-11-26T16:37:00-05:00" level=info msg="RPC server listening on 127.0.0.1:8082"
time="2023-11-26T16:37:00-05:00" level=info msg="BTC Validator Daemon is fully active!"
```

All the available cli options can be viewed using the `--help` flag. These options
can also be set in the configuration file.
