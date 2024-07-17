import {writeFileSync} from 'fs';
import path from 'path';

const OUTPUT_CONTRACT_ADDRESS_JSON_PATH = './addresses.json';
const OUTPUT_ENV_FILE_PATH = `./.contracts.env`;

function deepDelete(object: any, keyToDelete: string) {
  Object.keys(object).forEach(key => {
    if (key === keyToDelete) delete object[key];
    else if (typeof object[key] === 'object') deepDelete(object[key], keyToDelete);
  });
}

const transformData = (data: any) => {
  const result: {[key: string]: string} = {};

  Object.keys(data).forEach(chainId => {
    const networks = data[chainId];
    networks.forEach((network: any) => {
      const networkName = network.name;
      const contracts = network.contracts;
      Object.keys(contracts).forEach(contractName => {
        const contractAddress = contracts[contractName].address;
        result[`${networkName}-${contractName}`] = contractAddress;
      });
    });
  });

  return result;
};

function Main() {
  const keyToDelete = 'abi';

  const args = process.argv;
  const renameEnvVariablesJsonPath = args[2];

  let renameEnvVariablesJson: {[key: string]: string} = {};
  let isRenameEnvVariablesJsonPresent = false;

  const outputContractAddressJsonPathAbs = path.resolve(OUTPUT_CONTRACT_ADDRESS_JSON_PATH);
  const outputEnvFilePathAbs = path.resolve(OUTPUT_ENV_FILE_PATH);

  if (renameEnvVariablesJsonPath) {
    const envVariablesJsonPathAbs = path.resolve(renameEnvVariablesJsonPath);
    renameEnvVariablesJson = require(envVariablesJsonPathAbs);
    isRenameEnvVariablesJsonPresent = true;
  }

  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const contractAddresses = require(outputContractAddressJsonPathAbs);

  deepDelete(contractAddresses, keyToDelete);
  writeFileSync(outputContractAddressJsonPathAbs, JSON.stringify(contractAddresses, null, 2));
  console.log('Contract informations written to', outputContractAddressJsonPathAbs);

  const contractAddresssObj = transformData(contractAddresses);

  let outputEnvString = '';
  Object.keys(contractAddresssObj).forEach(contractName => {
    const contractAddress = contractAddresssObj[contractName];

    if (isRenameEnvVariablesJsonPresent && renameEnvVariablesJson.hasOwnProperty(contractName)) {
      const envValue = renameEnvVariablesJson[contractName];
      outputEnvString = `${outputEnvString}export ${envValue}=${contractAddress}\n`;
    } else {
      const envValue = contractName.toUpperCase().replace('-', '_');
      outputEnvString = `${outputEnvString}export ${envValue}=${contractAddress}\n`;
    }
  });

  writeFileSync(outputEnvFilePathAbs, outputEnvString);
  console.log('Contract address as environment variables written to', outputEnvFilePathAbs);
}

Main();
