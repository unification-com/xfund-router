const flatten = require("truffle-flattener")
const fs = require("fs")

const contractsToFlatten = [
  {
    path: "contracts",
    fileName: "Router.sol",
  },
  {
    path: "contracts/mocks",
    fileName: "xFUNDTestnet.sol",
  },
]

contractsToFlatten.forEach(async (c) => {
  const source = `./${c.path}/${c.fileName}`
  const dest = `./flat/${c.fileName}`
  const flat = await flatten([source])
  fs.writeFileSync(dest, flat)
})
