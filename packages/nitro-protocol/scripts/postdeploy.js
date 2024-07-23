"use strict";
exports.__esModule = true;
var fs_1 = require("fs");
var CONTRACT_ENV_MAP = {
    ConsensusApp: 'CA_ADDRESS',
    NitroAdjudicator: 'NA_ADDRESS',
    VirtualPaymentApp: 'VPA_ADDRESS',
    Bridge: 'BRIDGE_ADDRESS'
};
function deepDelete(object, keyToDelete) {
    Object.keys(object).forEach(function (key) {
        if (key === keyToDelete)
            delete object[key];
        else if (typeof object[key] === 'object')
            deepDelete(object[key], keyToDelete);
    });
}
function deepSearch(object, keyToSearch) {
    for (var key in object) {
        if (object.hasOwnProperty(key)) {
            if (key === keyToSearch) {
                return object[key].address;
            }
            else if (typeof object[key] === 'object') {
                var result = deepSearch(object[key], keyToSearch);
                if (result !== null) {
                    return result;
                }
            }
        }
    }
    return null;
}
function createEnvForContractAddresses(contractAddresses) {
    var outputEnvString = '';
    Object.entries(CONTRACT_ENV_MAP).forEach(function (_a) {
        var contractAddress = _a[0], envKey = _a[1];
        var envValue = deepSearch(contractAddresses, contractAddress);
        outputEnvString += "export ".concat(envKey, "=").concat(envValue, "\n");
    });
    return outputEnvString;
}
var jsonPath = __dirname + '/../addresses.json';
var contractEnvPath = __dirname + '/../.contracts.env';
// eslint-disable-next-line @typescript-eslint/no-var-requires
var addresses = require(jsonPath);
var keyToDelete = 'abi';
deepDelete(addresses, keyToDelete);
(0, fs_1.writeFileSync)(jsonPath, JSON.stringify(addresses, null, 2));
var envData = createEnvForContractAddresses(addresses);
(0, fs_1.writeFileSync)(contractEnvPath, envData);
