import 'hardhat-deploy';
import 'hardhat-deploy-ethers';
import fs from 'fs';
import path from 'path';

import {HardhatRuntimeEnvironment} from 'hardhat/types';
import {DeployFunction} from 'hardhat-deploy/types';

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const {deployments, getNamedAccounts, getChainId, network} = hre;
  const {deploy} = deployments;
  const {deployer} = await getNamedAccounts();

  const addressesFilePath = `hardhat-deployments/${network.name}/.contracts.env`;
  let contractAddresses = '';

  console.log('Working on chain id #', await getChainId());

  const caDeployResult = await deploy('ConsensusApp', {
    from: deployer,
    log: true,
    // TODO: Set ownership when using deterministic deployment
    deterministicDeployment: process.env.DISABLE_DETERMINISTIC_DEPLOYMENT ? false : true,
  });
  contractAddresses = `${contractAddresses}export CA_ADDRESS=${caDeployResult.address}\n`;

  const vpaDeployResult = await deploy('VirtualPaymentApp', {
    from: deployer,
    log: true,
    // TODO: Set ownership when using deterministic deployment
    deterministicDeployment: process.env.DISABLE_DETERMINISTIC_DEPLOYMENT ? false : true,
  });
  contractAddresses = `${contractAddresses}export VPA_ADDRESS=${vpaDeployResult.address}\n`;

  // TODO: Write to output file in post deploy script
  const outputFilePath = path.resolve(addressesFilePath);
  fs.writeFileSync(outputFilePath, contractAddresses, {flag: 'a'});
  console.log(
    'Consesus app and virtual payment app contracts deployed, addresses written to',
    outputFilePath
  );
};
export default func;
func.tags = ['deploy'];