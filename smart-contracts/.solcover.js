module.exports = {
  skipFiles: [
    "examples/DemoConsumer.sol",
    "examples/DemoConsumer2.sol",
    "mocks/bad/MockBadConsumer.sol",
    "mocks/bad/MockBadConsumerBigReceive.sol",
    "mocks/bad/MockBadConsumerInfiniteGas.sol",
    "mocks/MockConsumer.sol",
    "mocks/MockConsumerCustomRequest.sol",
    "mocks/xFUNDTestnet.sol",
    "vendor/OOOSafeMath.sol"],
  providerOptions: {
    mnemonic: "myth like bonus scare over problem client lizard pioneer submit female collect",
    gasPrice: "0x4A817C800",
    default_balance_ether: 100000,
  },
  mocha: {
    enableTimeouts: false
  }
}
