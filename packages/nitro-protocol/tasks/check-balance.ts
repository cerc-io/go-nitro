import {task} from 'hardhat/config';
import {ethers} from 'ethers';

export default task('check-token-balance', 'Prints the token balance of a given address')
  .addParam('token', 'The ERC-20 contract address')
  .addParam('address', 'The account address to check the balance for')
  .setAction(async (taskArgs, hre) => {
    const tokenAddress = taskArgs.token;
    const accountAddress = taskArgs.address;

    // Get the signer or provider
    const provider = hre.ethers.provider;

    // Get the ERC-20 contract instance
    const tokenContract = await hre.ethers.getContractAt(
      'ERC20',
      tokenAddress,
      provider.getSigner()
    );

    // Fetch the token balance
    const balance = await tokenContract.balanceOf(accountAddress);

    // Get token decimals to format the balance properly
    const decimals = await tokenContract.decimals();

    console.log(balance);

    // Convert balance from smallest unit (like Wei) to token units
    const formattedBalance = ethers.utils.formatUnits(balance, decimals);

    // Print the token balance
    console.log(
      `Balance of ${accountAddress}: ${formattedBalance} tokens with token address ${tokenAddress}`
    );
  });
