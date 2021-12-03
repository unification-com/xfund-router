# xFUND Router & Data Consumer Solidity Smart Contracts

[![npm version](http://img.shields.io/npm/v/@unification-com/xfund-router.svg?style=flat)](https://npmjs.org/package/@unification-com/xfund-router "View this project on npm")
![sc unit tests](https://github.com/unification-com/xfund-router/actions/workflows/test-contracts.yml/badge.svg)

A suite of smart contracts to enable data from external sources (such as Finchains.io)
to be included in your smart contracts. The suite comprises of:

1) A deployed Router smart contract. This facilitates receiving and forwarding data requests,
   between Consumers and Providers, in addition to processing xFUND payments for data provision.
2) A ConsumerBase smart contract, which is integrated into your own smart contract in 
   order for data requests to be initialised (via the Router), and data to be received (from
   a designated Provider)
   
The remainder of this `README` is aimed at developers who wish to develop and test the suite itself.
For an integration guide, and how to use the suite in your own smart contracts, please
see the [Documentation](docs/index.md)

## Development and Testing

### Prerequisites

#### NodeJS
The `openzeppelin/test-environment` packages and dependencies require
NodeJS >= `v8.9.4` and <= `v12.18.3` (excluding `v11`) in order to correctly install. 
We recommend using [nvm](https://github.com/nvm-sh/nvm) to manage NodeJS 
installations.

#### Yarn

[Yarn](https://classic.yarnpkg.com/en/docs/install) is recommended for package management.

### Compile

Run:

```bash
yarn install
```

to install the Node packages and dependencies

Run:
```bash 
npx truffle compile
```

to compile smart contracts

### Unit Tests

To run all tests:

```bash
yarn test
```

Running all tests will take a few minutes.

To run individual test files:

```bash
npx truffle test/[TEST_FILE]
```

Test Coverage

```bash
yarn run coverage
```

Running unit test coverage will take a long time. Results are saved to 
`converage.json` and `./coverage`

### Deployment & Interaction

Compile the contracts, if not already compiled:

```
npx truffle compile
```

Run the `truffle` development console:

```bash
npx truffle develop
```

**Note**: Make a note of the address and private key for `account[1]`. This will be
required later for Data Provider interaction.

#### Deploy the smart contracts

Run the `truffle` migrations, within the `development` console:

```bash
truffle(develop)> migrate
```

#### Interaction - as a Consumer

Within the `truffle` development console, load the contract instances, and accounts
ready for interaction

```bash 
truffle(develop)> let accounts = await web3.eth.getAccounts()
truffle(develop)> let mockToken = await MockToken.deployed()
truffle(develop)> let mockConsumer = await MockConsumer.deployed()
truffle(develop)> let router = await Router.deployed()
truffle(develop)> let consumerOwner = accounts[0]
truffle(develop)> let provider = accounts[1]
```

##### Initialisation

1. Grab some tokens from the `MockToken` smart contract for the `consumerOwner`. Run:

```bash
truffle(develop)> mockToken.gimme({from: consumerOwner})
```

2. Increase the `Router` Token allowance for the `MockConsumer` contract, so that the Router
   can hold and forward fees to the provider. Run:

```bash
truffle(develop)> mockConsumer.increaseRouterAllowance("115792089237316195423570985008687907853269984665640564039457584007913129639935", {from: consumerOwner})
```

3. Have the `provider` register with `Router`, with a minimum fee of 0.1 `xFUNDMOCK`:

```bash
truffle(develop)> router.registerAsProvider(100000000, {from: provider})
```

4. Transfer some `MOCK` tokens to your `MockConsumer` smart contract. Run:

```bash
truffle(develop)> mockToken.transfer(mockConsumer.address, 10000000000, {from: consumerOwner})
```

This will send 10 MOCKs to the MockConsumer smart contract.

##### Requesting Data

First, check the current `price` in your `MockConsumer` contract. Run:

```bash
truffle(develop)> let priceBefore = await mockConsumer.price()
truffle(develop)> priceBefore.toString()
```

The result should be 0.

Next, request some data from the provider. Run:

```bash
truffle(develop)> let endpoint = web3.utils.asciiToHex("BTC.GBP.PR.AVC.24H")
truffle(develop)> mockConsumer.getData(provider, 100000000, endpoint, {from: consumerOwner})
```

#### Interaction - as a Provider

**Note: the NodeJS implementation will eventually be deprecated in favour of the new Go implementation**

A Consumer requests data, but a provider Oracle needs to be running in order to fulfill
requests.

1. Copy `example.provider.env` to a `.env` file, and modify the following values. The
   `CONTRACT_ADDRESS` is the Router smart contract's address, and can be acquired
   by running:
   
```bash 
truffle(develop)> router.address
```

The values for `WALLET_PKEY` and `WALLET_ADDRESS` are whatever you noted down when initialising 
`truffle develop` earlier.

``` 
CONTRACT_ADDRESS=
WALLET_PKEY=
WALLET_ADDRESS=
```

**Note:** the `CONTRACT_ADDRESS` should be the address of the `Router` smart contract.

In a separate terminal, run:

```bash
npx sequelize-cli db:migrate
```

to initialise the oracle's database. If required, run

```bash
npx sequelize-cli db:migrate:undo:all
```

first to wipe the database.

**Note**: the default `NODE_ENV` environment is `development`. In the `development` environment,
the Oracle will initialise a local SQLite database and save it to `provider-oracle/db/database.sqlite`.
This is intended for rapid testing/development, and is not meant for extensive testing. To test
in a more realistic environment, install PostgreSQL and change the `NODE_ENV` in `.env`
to `test`. Configure the appropriate `DB_` values in `.env` and re-run the above `sequelize`
commands to initialise the PostgreSQL database.

Run the oracle with:

``` 
yarn run dev:oracle
```

This will run, watching the `Router` smart contract for any `DataRequested` events. If
one is emitted, and the `dataProvider` value is the provider's wallet address, it will
grab the data, and call the `Router`'s `fulfillRequest` function, which in turn forwards
the data to the requesting `MockConsumer` smart contract.

``` 
2021-03-08T11:36:23.293Z current Eth height 15
2021-03-08T11:36:23.302Z get supported pairs
2021-03-08T11:36:37.055Z watching DataRequested from block 0
2021-03-08T11:36:37.055Z BEGIN watchIncommingRequests
2021-03-08T11:36:37.056Z running watcher for DataRequested
2021-03-08T11:36:37.065Z watching RequestFulfilled from block 0
2021-03-08T11:36:37.066Z BEGIN watchIncommingFulfillments
2021-03-08T11:36:37.066Z running watcher for RequestFulfilled
2021-03-08T11:36:37.067Z watching RequestCancelled from block 0
2021-03-08T11:36:37.068Z BEGIN watchIncommingCancellations
2021-03-08T11:36:37.068Z running watcher for RequestCancelled
2021-03-08T11:36:37.069Z watching blocks for jobs to process from block 0
2021-03-08T11:36:37.070Z BEGIN fulfillRequests
2021-03-08T11:36:37.081Z running block watcher
2021-03-08T11:36:37.135Z watchEvent DataRequested connected 0x1
2021-03-08T11:36:37.136Z watchEvent RequestFulfilled connected 0x2
2021-03-08T11:36:37.142Z watchBlocks newBlockHeaders connected 0x3
```

**Note**: The price returned by the Oracle is always standardised to `actualPrice * (10 ** 18)`

##### Check Price in `MockConsumer`

Depending on what is configured for the `WAIT_CONFIRMATIONS` in `.env`, you will need
to run the following command in the Truffle console a few times to mine blocks:

```bash
truffle(develop)> await web3.currentProvider.send({ jsonrpc: "2.0", method: "evm_mine", id: 12345 }, function(err, result) { return })
```

This will force `ganache-cli` to mine new blocks until the required number of block confirmations
have been achieved for the Provider Oracle to process and fulfil the data request.

Once the provider has fulfilled the request, the `price` value should have been updated
in the `MockConsumer` smart contract. Run:

```bash
truffle(develop)> let priceAfter = await mockConsumer.price()
truffle(develop)> priceAfter.toString()
```

The result should now be a non-zero
value, e.g. `36547117907180545000000`.

## Testing the go-ooo Oracle implementation

```bash
make dev-env
make build
./go-ooo/build/go-ooo init
./go-ooo/build/go-ooo start --pass /path/to/pass.txt
```

Request data

```bash
docker exec -it ooo_dev_env /root/xfund-router/request.sh BTC GBP PR AVC 1H
docker exec -it ooo_dev_env /root/xfund-router/request.sh LEASH WETH AD
```

#### Dev Notes

Verify contracts on Etherscan after deployment - set `ETHERSCAN_API` in `.env`, then run:

```bash 
npx truffle run verify [ContractName] --network=[network]
```
