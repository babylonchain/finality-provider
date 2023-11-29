# EOTS daemon (`eotsd`) configuration:

The `eotscli` tool serve as control planes for the EOTS Daemon (`eotsd`). Below,
instructions are provided for configuring the `eotsd` daemon.

The `eotscli admin dump-config` command initializes a home directory for the EOTS
manager. This directory is created in the default home location or in a location
specified by the `--config-file-dir` flag.

```bash
$ eotscli admin dump-config --config-file-dir /path/to/eotsd-home/
```

After initialization, the home directory will have the following structure

```bash
$ ls /path/to/eotsd-home/
  ├── eotsd.conf # Eotsd-specific configuration file.
```

If the `--config-file-dir` flag is not specified, then the default home location will
be used. For different operating systems, those are:

- **MacOS** `~/Library/Application Support/Eotsd`
- **Linux** `~/.Eotsd`
- **Windows** `C:\Users\<username>\AppData\Local\Eotsd`

Below are some of the important parameters in the `eotsd.conf` file.

```bash
# Path to EOTSD configuration file
ConfigFile = /Users/<user>/Library/Application Support/Eotsd/eotsd.conf

# Default address to listen for RPC connections
RpcListener = localhost:15813

# Directory to store EOTS manager keys
KeyDirectory = /Users/<user>/Library/Application Support/Eotsd/data

# Type of keyring to use
KeyringBackend = file

# Possible database to choose as backend
Backend = bbolt

# Path to the database
Path = bbolt-eots.db

# Name of the database
Name = default
```

To see the complete list of configuration options, check the `eotsd.conf` file.
