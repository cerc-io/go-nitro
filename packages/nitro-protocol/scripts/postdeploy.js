var writeFileSync = require('fs').writeFileSync;
var path = require('path');
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
function createEnvForContractAddresses(contractAddresses) {
    for (var key in contractAddresses) {
        var networkArray = contractAddresses[key];
        var _loop_1 = function (network) {
            var networkName = network.name;
            var contractDetails = network.contracts;
            var envToWrite = '';
            var envFilePath = "./hardhat-deployments/".concat(networkName, "/.contracts.env");
            Object.entries(contractDetails).forEach(function (_a) {
                var contractName = _a[0], value = _a[1];
                var envValue = value.address;
                var envName = contractName;
                if (CONTRACT_ENV_MAP.hasOwnProperty(contractName)) {
                    envName = CONTRACT_ENV_MAP[contractName];
                }
                envToWrite += "export ".concat(envName, "=").concat(envValue, "\n");
            });
            var outputFilePath = path.resolve(envFilePath);
            writeFileSync(outputFilePath, envToWrite);
        };
        for (var _i = 0, networkArray_1 = networkArray; _i < networkArray_1.length; _i++) {
            var network = networkArray_1[_i];
            _loop_1(network);
        }
    }
}
var jsonPath = __dirname + '/../addresses.json';
// eslint-disable-next-line @typescript-eslint/no-var-requires
var addresses = require(jsonPath);
var keyToDelete = 'abi';
deepDelete(addresses, keyToDelete);
writeFileSync(jsonPath, JSON.stringify(addresses, null, 2));
createEnvForContractAddresses(addresses);
