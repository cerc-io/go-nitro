// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;
import {ERC20} from '@openzeppelin/contracts/token/ERC20/ERC20.sol';

/**
 * @dev This contract extends an ERC20 implementation, and mints 10,000,000,000 tokens to the deploying account. Used for testing purposes.
 */
contract Token is ERC20 {
    /**
     * @dev Constructor function minting 10 billion tokens to the owner. Do not use msg.sender for default owner as that will not work with CREATE2
     * @param name Name of the token
     * @param symbol Symbol of the token
     * @param owner Tokens are minted to the owner address
     * @param initialSupply Initial supply of tokens
     */
    constructor(
        string memory name,
        string memory symbol,
        address owner,
        uint256 initialSupply
    ) ERC20(name, symbol) {
        _mint(owner, initialSupply);
    }
}
