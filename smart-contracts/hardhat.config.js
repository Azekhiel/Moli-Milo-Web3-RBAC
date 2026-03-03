require("@nomiclabs/hardhat-waffle"); 

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.24",
  
  networks: {
    gethDev: {
      url: "http://localhost:8545",
    }
  }
};