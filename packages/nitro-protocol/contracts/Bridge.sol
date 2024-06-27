// SPDX-License-Identifier: MIT
pragma solidity 0.8.17;
import {StatusManager} from './StatusManager.sol';
import {IBridge} from './interfaces/IBridge.sol';

contract Bridge is StatusManager, IBridge {
  // Owner only method to update mirrored ledger channel state
  function updateMirroredChannelStatus( bytes32 channelId, bytes32 stateHash,bytes memory outcomeBytes) public virtual onlyOwner {
  _updateFingerprint(channelId, stateHash, keccak256(outcomeBytes));
    emit StatusUpdated(channelId, stateHash);
  }

  function _updateFingerprint(
    bytes32 channelId,
    bytes32 stateHash,
    bytes32 outcomeHash
  ) internal {
    (uint48 turnNumRecord, uint48 finalizesAt, ) = _unpackStatus(channelId);

    bytes32 newStatus = _generateStatus(
        ChannelData(turnNumRecord, finalizesAt, stateHash, outcomeHash)
    );
    statusOf[channelId] = newStatus;
  }

  // Public getter function to get ledger channel state
  function getMirroredChannelStatus(bytes32 channelId) public view returns (bytes32) {
    return statusOf[channelId];
  }
}
