import {HardhatRuntimeEnvironment} from 'hardhat/types';
import {DeployFunction} from 'hardhat-deploy/types';

import {
  DEFAULT_INITIAL_TOKEN_SUPPLY,
  DEFAULT_TOKEN_NAME,
  DEFAULT_TOKEN_SYMBOL,
} from '../src/constants';

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const {
    deployments: {deploy},
    getNamedAccounts,
  } = hre;

  const {deployer} = await getNamedAccounts();

  const name = process.env.TOKEN_NAME || DEFAULT_TOKEN_NAME;
  const symbol = process.env.TOKEN_SYMBOL || DEFAULT_TOKEN_SYMBOL;
  const initialSupply = process.env.INITIAL_TOKEN_SUPPLY
    ? hre.ethers.utils.parseUnits(process.env.INITIAL_TOKEN_SUPPLY, 18)
    : DEFAULT_INITIAL_TOKEN_SUPPLY;

  await deploy('Token', {
    from: deployer,
    log: true,
    args: [name, symbol, deployer, initialSupply],
    // TODO: Set ownership when using deterministic deployment
    deterministicDeployment: process.env.DISABLE_DETERMINISTIC_DEPLOYMENT ? false : true,
  });
};
export default func;
func.tags = ['Token'];
