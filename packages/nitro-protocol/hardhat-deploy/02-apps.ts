import 'hardhat-deploy';
import 'hardhat-deploy-ethers';

import {HardhatRuntimeEnvironment} from 'hardhat/types';
import {DeployFunction} from 'hardhat-deploy/types';

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const {deployments, getNamedAccounts, getChainId} = hre;
  const {deploy} = deployments;
  const {deployer} = await getNamedAccounts();

  console.log('Working on chain id #', await getChainId());

  await deploy('ConsensusApp', {
    from: deployer,
    log: true,
    // TODO: Set ownership when using deterministic deployment
    deterministicDeployment: process.env.DISABLE_DETERMINISTIC_DEPLOYMENT ? false : true,
  });

  await deploy('VirtualPaymentApp', {
    from: deployer,
    log: true,
    // TODO: Set ownership when using deterministic deployment
    deterministicDeployment: process.env.DISABLE_DETERMINISTIC_DEPLOYMENT ? false : true,
  });
};
export default func;
func.tags = ['deploy'];
