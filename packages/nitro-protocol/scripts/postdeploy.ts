const { writeFileSync } = require('fs');
const path = require('path');

type Contract = {
  address: string;
};

type Chain = {
  name: string;
  chainId: string;
  contracts: Record<string, Contract>;
};

type ContractDetails = Record<string, Chain[]>;

const CONTRACT_ENV_MAP: {[key: string]: string} = {
  ConsensusApp: 'CA_ADDRESS',
  NitroAdjudicator: 'NA_ADDRESS',
  VirtualPaymentApp: 'VPA_ADDRESS',
  Bridge: 'BRIDGE_ADDRESS',
};

function deepDelete(object: any, keyToDelete: string) {
  Object.keys(object).forEach(key => {
    if (key === keyToDelete) delete object[key];
    else if (typeof object[key] === 'object') deepDelete(object[key], keyToDelete);
  });
}

function createEnvForContractAddresses(contractAddresses: ContractDetails) {
  for (const key in contractAddresses) {
    const networkArray = contractAddresses[key];
    for (const network of networkArray) {
      const networkName = network.name;
      const contractDetails = network.contracts;
      let envToWrite = '';
      const envFilePath = `./hardhat-deployments/${networkName}/.contracts.env`;

      Object.entries(contractDetails).forEach(([contractName, value]) => {
        const envValue = value.address;
        let envName = contractName;

        if (CONTRACT_ENV_MAP.hasOwnProperty(contractName)) {
          envName = CONTRACT_ENV_MAP[contractName];
        } 

        envToWrite += `export ${envName}=${envValue}\n`;
      });

      const outputFilePath = path.resolve(envFilePath);
      writeFileSync(outputFilePath, envToWrite);
    }
  } 
}

const jsonPath = __dirname + '/../addresses.json';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const addresses = require(jsonPath);

const keyToDelete = 'abi';
deepDelete(addresses, keyToDelete);
writeFileSync(jsonPath, JSON.stringify(addresses, null, 2));
createEnvForContractAddresses(addresses);
