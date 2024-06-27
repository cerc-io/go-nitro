// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;

interface IBridge {
  // Method to update mirrored ledger channel state
  function updateMirroredChannelStatus( bytes32 channelId, bytes32 stateHash,bytes memory outcomeBytes) external;

  function getMirroredChannelStatus(bytes32 channelId) external view returns (bytes32);

  event StatusUpdated(bytes32 indexed channelId, bytes32 stateHash);
}
