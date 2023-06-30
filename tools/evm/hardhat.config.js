/** @type import('hardhat/config').HardhatUserConfig */

require("@nomiclabs/hardhat-web3");
require("@nomiclabs/hardhat-ethers");

module.exports = {
  solidity: "0.8.18",
  web3: {
    personal: true,
  },

  networks: {
    hardhat: {
      // Accounts correspond to those created by smoke tests and evm-tool.py.
      accounts: [
        {
          privateKey:
            "0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027",
          balance: "100000000000000000000",
        },
        {
          privateKey:
            "0x289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
          balance: "100000000000000000000",
        },
        {
          privateKey:
            "0xe810f1d7d6691b4a7a73476f3543bd87d601f9a53e7faf670eac2c5b517d83bf",
          balance: "100000000000000000000",
        },
        {
          privateKey:
            "0xa96e62ed3955e65be32703f12d87b6b5cf26039ecfa948dc5107a495418e5330",
          balance: "100000000000000000000",
        },
        {
          privateKey:
            "0x9294f4d108465fd293f7fe299e6923ef71a77f2cb1eb6d4394839c64ec25d5c0",
          balance: "100000000000000000000",
        },
      ],
    },
  },
};
