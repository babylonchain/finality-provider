# Installation

## Prerequisites

This project requires Go version 1.20 or later.

Install Go by following the instructions on the [official Go installation guide](https://golang.org/doc/install).

## Downloading the code

To get started, clone the repository to your local machine from Github:

```bash
git clone git@github.com:babylonchain/btc-validator.git
```

You can choose a specific version from the [official releases page](https://github.com/babylonchain/btc-validator/releases)

```bash
cd btc-validator # cd into the project directory
git checkout <release-tag>
```

## Building and installing the binary

```bash
cd btc-validator # cd into the project directory
make build # build the binaries in the build directory
make install # install the binaries to your $GOPATH/bin directory
```

The build directory has the following structure
```bash
ls build
    ├── eotcli
    ├── eotsd
    ├── valcli
    └── vald
```
