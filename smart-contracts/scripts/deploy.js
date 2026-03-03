const { ethers } = require("hardhat");

async function main() {
  console.log("Memulai proses deployment...");

  const [deployer] = await ethers.getSigners();
  
  console.log(`Deploying kontrak dengan akun: ${deployer.address}`);
  console.log(`(Seharusnya 0x7156...17f7)`);
  console.log(`Saldo akun: ${(await deployer.getBalance()).toString()} Wei`);

  // Ambil 'blueprint' kontrak
  const ContractFactory = await ethers.getContractFactory("AuditableRBAC", deployer);

  console.log("Mengirim transaksi deployment...");
  const rbacContract = await ContractFactory.deploy();

  await rbacContract.deployed();

  console.log(
    `Kontrak AuditableRBAC berhasil di-deploy ke alamat: ${rbacContract.address}`
  );
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});