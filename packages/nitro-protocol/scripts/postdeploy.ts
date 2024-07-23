import {writeFileSync} from 'fs';

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

function deepSearch(object: any, keyToSearch: string): string | null {
  for (const key in object) {
    if (object.hasOwnProperty(key)) {
      if (key === keyToSearch) {
        return object[key].address;
      } else if (typeof object[key] === 'object') {
        const result = deepSearch(object[key], keyToSearch);
        if (result !== null) {
          return result;
        }
      }
    }
  }

  return null;
}

function createEnvForContractAddresses(contractAddresses: any): string {
  let outputEnvString = '';

  Object.entries(CONTRACT_ENV_MAP).forEach(([contractAddress, envKey]) => {
    const envValue = deepSearch(contractAddresses, contractAddress);
    outputEnvString += `export ${envKey}=${envValue}\n`;
  });

  return outputEnvString;
}

const jsonPath = __dirname + '/../addresses.json';
const contractEnvPath = __dirname + '/../.contracts.env';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const addresses = require(jsonPath);

const keyToDelete = 'abi';
deepDelete(addresses, keyToDelete);
writeFileSync(jsonPath, JSON.stringify(addresses, null, 2));
const envData = createEnvForContractAddresses(addresses);
writeFileSync(contractEnvPath, envData);
