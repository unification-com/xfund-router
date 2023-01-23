# go-ooo

The Go implementation of the OoO Provider application, required to be run by providers to serve data

## Development and Testing

### Prerequisites

#### Go

Go v1.18+ is required to compile the `go-ooo` application.

## Running

First, build the Go application:

```bash
make build
```

`go-ooo` will need initialising before it can run:

```bash
./build/go-ooo init <network>
```

Where `<network>` is one of `dev`, `goerli`, `mainnet` or `polygon`. Using `dev` will configure `go-ooo` for the Docker
[development environment](../docker/README.md).

This will save the default configuration to `$HOME/.go-ooo`, with the initial values for the `dev` network.
This config location can be changed using the `--home` flag to specify a custom location, e.g.

```bash
./build/go-ooo init dev --home $HOME/.go-ooo_dev
```

This initialisation script will ask whether you want to import an exisitng private key, or generate a new one.
You can enter anything for the account name. For the purposes of quick testing, the Docker development environment
initialises by pre-registering account #3 on the `ganache-cli` chain as a Provider Oracle. The private key to import is:

`0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913`

Make a note of the generated decryption password - this will be required to run the application (to decrypt the keystore)
and to execute any admin commands. For the sake of simplicity, save it to `$HOME/.go-ooo/pass.txt`

The application should now have the default configuration saved to `$HOME/.go-ooo/config.toml`. It will use `sqlite` as
the default database, but can easily be configured for PostgreSQL.

#### Registering a new Oracle Provider

If `go-ooo` has been initialised for a network other than `dev`, and using a key other than the pre-defined test key,
then registration as an Oracle Provider is required. First, ensure the wallet being used has funds on the target
chain, then run the registration admin command:

```bash
./build/go-ooo admin register [FEE] --home /path/to/.go-ooo --pass /path/to/pass.txt
```

Where `[FEE]` is your fee, for example `1000000` for 0.001 xFUND.

#### Start the Oracle

Now, you can start the Provider Oracle:

```bash
./build/go-ooo start
```

This will prompt you for the decryption password, and start the application. If you saved the password, you can pass the
path to the file using the `--pass` flag, e.g.

```bash
./build/go-ooo start --home $HOME/.go-ooo_dev --pass $HOME/.go-ooo_dev/pass.txt
```

## Docker Developer Environment

If the [Developer Environment](../docker/README.md) is running, these will have been deployed automatically, along with
a number of funded test accounts for both an OoO Provider and end-users. The `dev` network provider above should be 
pre-registered in the Router smart contract.
