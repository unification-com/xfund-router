require("dotenv").config()
const HDWalletProvider = require("@truffle/hdwallet-provider")

const { ETH_PKEY } = process.env

module.exports = {
  customNetworks: {
    my_custom_net: {
      provider: () =>
        new HDWalletProvider({
          privateKeys: [ETH_PKEY],
          providerOrUrl: "https://URL:PORT",
        }),
      network_id: "12345",
      skipDryRun: true,
    },
  },
}
