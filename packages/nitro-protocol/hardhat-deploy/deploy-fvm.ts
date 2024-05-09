import 'hardhat-deploy';
import 'hardhat-deploy-ethers';
import fs from 'fs';
import path from 'path';

import {HardhatRuntimeEnvironment} from 'hardhat/types';

module.exports = async (hre: HardhatRuntimeEnvironment) => {
  const {deployments, getNamedAccounts, getChainId, ethers, network} = hre;
  const {deploy} = deployments;
  const {deployer} = await getNamedAccounts();

  const addressesFilePath = `hardhat-deployments/${network.name}/.contracts.env`;
  let contractAddresses = '';

  console.log('Working on chain id #', await getChainId());
  console.log('deployer', deployer);

  try {
    const deployResult = await deploy('NitroAdjudicator', {
      from: deployer,
      args: [],
      // since Ethereum's legacy transaction format is not supported on FVM, we need to specify
      // maxPriorityFeePerGas to instruct hardhat to use EIP-1559 tx format
      maxPriorityFeePerGas: ethers.BigNumber.from(1500000000),
      maxFeePerGas: ethers.BigNumber.from(1500000000),
      skipIfAlreadyDeployed: false,
      log: true,
    });
    contractAddresses = `${contractAddresses}NA_ADDRESS=${deployResult.address}\n`;
  } catch (err) {
    const msg = err instanceof Error ? err.message : JSON.stringify(err);
    console.error(`Error when deploying contract: ${msg}`);
  }

  try {
    const deployResult = await deploy('ConsensusApp', {
      from: deployer,
      args: [],
      // since Ethereum's legacy transaction format is not supported on FVM, we need to specify
      // maxPriorityFeePerGas to instruct hardhat to use EIP-1559 tx format
      maxPriorityFeePerGas: ethers.BigNumber.from(1500000000),
      maxFeePerGas: ethers.BigNumber.from(1500000000),
      skipIfAlreadyDeployed: false,
      log: true,
    });
    contractAddresses = `${contractAddresses}CA_ADDRESS=${deployResult.address}\n`;
  } catch (err) {
    const msg = err instanceof Error ? err.message : JSON.stringify(err);
    console.error(`Error when deploying contract: ${msg}`);
  }

  try {
    const deployResult = await deploy('VirtualPaymentApp', {
      from: deployer,
      args: [],
      // since Ethereum's legacy transaction format is not supported on FVM, we need to specify
      // maxPriorityFeePerGas to instruct hardhat to use EIP-1559 tx format
      maxPriorityFeePerGas: ethers.BigNumber.from(1500000000),
      maxFeePerGas: ethers.BigNumber.from(1500000000),
      skipIfAlreadyDeployed: false,
      log: true,
    });
    contractAddresses = `${contractAddresses}VPA_ADDRESS=${deployResult.address}\n`;
  } catch (err) {
    const msg = err instanceof Error ? err.message : JSON.stringify(err);
    console.error(`Error when deploying contract: ${msg}`);
  }

  const outputFilePath = path.resolve(addressesFilePath);
  fs.writeFileSync(outputFilePath, contractAddresses);
  console.log('Contracts deployed, addresses written to', outputFilePath);
};
module.exports.tags = ['deploy-fvm'];
