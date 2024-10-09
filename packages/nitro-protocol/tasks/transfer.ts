import {task} from 'hardhat/config';

export default task('transfer', 'Transfers ERC20 tokens')
  .addParam('contract', 'The contract address')
  .addParam('to', 'The recipient address')
  .addParam('amount', 'The amount to transfer')
  .setAction(async (taskArgs, hre) => {
    const contractAddress = taskArgs.contract;
    const recipient = taskArgs.to;
    const amount = taskArgs.amount;

    // Get the signer (sender) to perform the transaction
    const [sender] = await hre.ethers.getSigners();

    // Get the ERC20 contract instance
    const token = await hre.ethers.getContractAt('Token', contractAddress, sender);

    // Parse the amount to transfer
    const parsedAmount = hre.ethers.utils.parseUnits(amount, 18);

    // Perform the transfer
    const tx = await token.transfer(recipient, parsedAmount);
    await tx.wait();

    console.log(
      `Transferred ${amount} tokens with address ${contractAddress} to ${recipient} from ${sender.address}`
    );
  });
