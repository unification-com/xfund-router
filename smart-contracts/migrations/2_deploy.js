const xFUNDTestnet = artifacts.require("xFUNDTestnet")
const Router = artifacts.require("Router")
const MockConsumer = artifacts.require("MockConsumer")
const DemoConsumer = artifacts.require("DemoConsumer2")

module.exports = function (deployer, network) {
  // deployment steps
  switch (network) {
    default:
    case "development":
    case "develop":
      // 1. xFUNDTestnet
      deployer
        .deploy(xFUNDTestnet, "xFUND", "xFUND", 10000000000000, 9)
        .then(function () {
          // 2. Router
          return deployer.deploy(Router, xFUNDTestnet.address)
        })
        .then(function () {
          // 3. MockConsumer
          return deployer.deploy(MockConsumer, Router.address, xFUNDTestnet.address)
        })
        .then(function () {
          // 4. MockConsumerCustomRequest
          return deployer.deploy(
            DemoConsumer,
            Router.address,
            xFUNDTestnet.address,
            "0xE11BA2b4D45Eaed5996Cd0823791E0C93114882d",
            "10000000",
          )
        })
      break
    case "mainnet":
    case "mainnet-fork":
      // Router
      deployer.deploy(Router, "0x892A6f9dF0147e5f079b0993F486F9acA3c87881")
      break
    case "polygon":
    case "polygon-fork":
      // Router
      deployer.deploy(Router, "0x77a3840f78e4685afaf9c416b36e6eae6122567b")
      break
    case "sepolia":
    case "sepolia-fork":
      // Router
      deployer.deploy(Router, "0xb07C72acF3D7A5E9dA28C56af6F93862f8cc8196")
      break
    case "shibarium":
    case "shibarium-fork":
      deployer.deploy(Router, "0x89dc93C6c12CaE47aCAf4aD9305d7A442C30dBB2")
      break
    case "puppynet":
    case "puppynet-fork":
      deployer.deploy(Router, "0x78f022230EaE6E05D8739E83a14b0Cf1D00CfaD5")
      break
    case "qom":
    case "qom-fork":
      deployer.deploy(Router, "0x0d2FDD551199513D89D64d766E053E6fBC838831")
      break
  }
}
