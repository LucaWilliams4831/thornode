const { ethers } = require("hardhat");
const net = require("net");

async function main() {
  // wait until port 5458 is open
  const host = "127.0.0.1";
  const port = 5458;
  let ready = false;
  while (true) {
    console.log("Waiting for hardhat node to start...");
    const client = net.createConnection({ host, port }, () => {
      ready = true;
      client.destroy();
    });
    client.on("error", () => {});
    await new Promise((resolve) => setTimeout(resolve, 1000));
    if (ready) {
      break;
    }
  }

  // account used as default in evm-tool.py and smoke tests
  const accountAddress = "0x8db97c7cece249c2b98bdc0226cc4c2a57bf52fc";

  // connect to hardhat node with impersonated account
  const provider = new ethers.getDefaultProvider("http://127.0.0.1:5458");
  await provider.send("hardhat_impersonateAccount", [accountAddress]);

  // list of USD tokens on various networks
  const usdTokens = {
    ETH: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // ETH.USDC
    AVAX: "0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E", // AVAX.USDC
    BSC: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", // BSC.USDC
  };

  for (const [network, token] of Object.entries(usdTokens)) {
    // skip tokens for other networks
    if ((await provider.getCode(token)) == "0x") {
      continue;
    }

    const erc20 = new ethers.Contract(
      token,
      ["function balanceOf(address) view returns (uint256)"],
      provider
    );
    const mintAmount = ethers.utils.parseUnits("100000", 6);

    // set the balance of the account to 100K USD
    switch (network) {
      case "AVAX":
      case "ETH": {
        // call mint function on the token contract
        const contract = new ethers.Contract(
          token,
          [
            "function balanceOf(address) view returns (uint256)",
            "function masterMinter() view returns (address)",
            "function mint(address,uint256)",
            "function configureMinter(address,uint256)",
          ],
          provider
        );

        // get minter and set balance
        const minter = await contract.masterMinter();
        await provider.send("hardhat_setBalance", [
          minter,
          ethers.utils.hexlify(ethers.utils.parseEther("10")),
        ]);

        // impersonate the master minter to grant minting rights
        await provider.send("hardhat_impersonateAccount", [minter]);
        const signer = await provider.getSigner(minter);
        const configContract = contract.connect(signer);
        await configContract.configureMinter(minter, mintAmount);

        // mint 100K USD to the account
        await configContract.mint(accountAddress, mintAmount);
      }

      case "BSC": {
        // call mint function on the token contract
        const contract = new ethers.Contract(
          token,
          [
            "function balanceOf(address) view returns (uint256)",
            "function mint(uint256)",
            "function getOwner() view returns (address)",
            "function transfer(address,uint256)",
          ],
          provider
        );

        // get owner
        const owner = await contract.getOwner();

        // impersonate the owner
        await provider.send("hardhat_impersonateAccount", [owner]);
        const signer = await provider.getSigner(owner);
        const configContract = contract.connect(signer);

        // mint 100K USD to the account
        await configContract.mint(mintAmount);
        await configContract.transfer(accountAddress, mintAmount);
      }
    }

    // print the token balance for the account
    const balance = await erc20.balanceOf(accountAddress);
    console.log(
      `${accountAddress}: ${ethers.utils.formatUnits(
        balance,
        6
      )} ${network}.USDC (${token})`
    );
  }
}

main();
