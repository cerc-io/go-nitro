import {Contract, constants, BigNumber} from 'ethers';
import {it} from '@jest/globals';
import {Allocation, AllocationType} from '@statechannels/exit-format';

import {Alice as AWallet, Bob as BWallet, TestChannel} from '../../../gas-benchmarks/fixtures';
import {
  getTestProvider,
  randomChannelId,
  randomExternalDestination,
  setupContract,
} from '../../test-helpers';
import {TESTNitroAdjudicator} from '../../../typechain-types/TESTNitroAdjudicator';
// eslint-disable-next-line import/order
import TESTNitroAdjudicatorArtifact from '../../../artifacts/contracts/test/TESTNitroAdjudicator.sol/TESTNitroAdjudicator.json';
import {
  channelDataToStatus,
  encodeOutcome,
  getVariablePart,
  hashOutcome,
  hashState,
  Outcome,
} from '../../../src';
import {MAGIC_ADDRESS_INDICATING_ETH} from '../../../src/transactions';
import {encodeGuaranteeData} from '../../../src/contract/outcome';
const provider = getTestProvider();

const testNitroAdjudicator: TESTNitroAdjudicator & Contract = setupContract(
  provider,
  TESTNitroAdjudicatorArtifact,
  process.env.TEST_NITRO_ADJUDICATOR_ADDRESS
) as unknown as TESTNitroAdjudicator & Contract;

// Amounts are valueString representations of wei
describe('reclaim', () => {
  // TODO: add a test case to show off a multihop reclaim, where we have Alice, Irene, Ivan and Bob.
  it('handles a simple case as expected', async () => {
    const targetId = randomChannelId();
    const sourceId = randomChannelId();
    const Alice = randomExternalDestination();
    const Bob = randomExternalDestination();
    const Irene = randomExternalDestination();

    // prepare an appropriate virtual channel outcome and finalize

    const vAllocations: Allocation[] = [
      {
        destination: Alice,
        amount: BigNumber.from(7).toHexString(),
        allocationType: AllocationType.simple,
        metadata: '0x',
      },
      {
        destination: Bob,
        amount: BigNumber.from(3).toHexString(),
        allocationType: AllocationType.simple,
        metadata: '0x',
      },
    ];

    const vOutcome: Outcome = [
      {
        asset: MAGIC_ADDRESS_INDICATING_ETH,
        allocations: vAllocations,
        assetMetadata: {assetType: 0, metadata: '0x'},
      },
    ];
    const vOutcomeHash = hashOutcome(vOutcome);
    await (
      await testNitroAdjudicator.setStatusFromChannelData(targetId, {
        turnNumRecord: 99,
        finalizesAt: 1,
        stateHash: constants.HashZero, // not realistic, but OK for purpose of this test
        outcomeHash: vOutcomeHash,
      })
    ).wait();

    // prepare an appropriate ledger channel outcome and finalize

    const lAllocations: Allocation[] = [
      {
        destination: Alice,
        amount: BigNumber.from(10).toHexString(),
        allocationType: AllocationType.simple,
        metadata: '0x',
      },
      {
        destination: Irene,
        amount: BigNumber.from(10).toHexString(),
        allocationType: AllocationType.simple,
        metadata: '0x',
      },
      {
        destination: targetId,
        amount: BigNumber.from(10).toHexString(),
        allocationType: AllocationType.guarantee,
        metadata: encodeGuaranteeData({left: Alice, right: Irene}),
      },
    ];

    const lChannel = new TestChannel('0x0', [AWallet, BWallet], lAllocations);

    const lOutcomeHash = hashOutcome(lChannel.outcome(MAGIC_ADDRESS_INDICATING_ETH));
    const lStateHash = hashState(lChannel.someState(MAGIC_ADDRESS_INDICATING_ETH));

    await (
      await testNitroAdjudicator.setStatusFromChannelData(sourceId, {
        turnNumRecord: lChannel.someState(MAGIC_ADDRESS_INDICATING_ETH).turnNum,
        finalizesAt: 1,
        stateHash: lStateHash, // not realistic, but OK for purpose of this test
        outcomeHash: lOutcomeHash,
      })
    ).wait();

    // call reclaim

    const tx = testNitroAdjudicator.reclaim({
      sourceChannelId: sourceId,
      fixedPart: lChannel.fixedPart,
      variablePart: getVariablePart(lChannel.someState(MAGIC_ADDRESS_INDICATING_ETH)),
      sourceOutcomeBytes: encodeOutcome(lChannel.outcome(MAGIC_ADDRESS_INDICATING_ETH)),
      sourceAssetIndex: 0, // TODO: introduce test cases with multiple-asset Source and Targets
      indexOfTargetInSource: 2,
      targetStateHash: constants.HashZero,
      targetOutcomeBytes: encodeOutcome(vOutcome),
      targetAssetIndex: 0,
    });

    // Extract logs
    const {events: eventsFromTx} = await (await tx).wait();

    // Compile event expectations

    // Check that each expectedEvent is contained as a subset of the properies of each *corresponding* event: i.e. the order matters!
    const expectedEvents = [
      {
        event: 'Reclaimed',
        args: {
          channelId: sourceId,
          assetIndex: BigNumber.from(0),
        },
      },
    ];

    expect(eventsFromTx).toMatchObject(expectedEvents);

    // assert on updated ledger channel

    // Check new outcomeHash
    const allocationAfter: Allocation[] = [
      {
        destination: Alice,
        amount: BigNumber.from(17).toHexString(),
        allocationType: AllocationType.simple,
        metadata: '0x',
      },
      {
        destination: Irene,
        amount: BigNumber.from(13).toHexString(),
        allocationType: AllocationType.simple,
        metadata: '0x',
      },
    ];

    const lChannelAfter = new TestChannel('0x0', [AWallet, BWallet], allocationAfter);
    const expectedStatusAfter = channelDataToStatus({
      turnNumRecord: lChannel.someState(MAGIC_ADDRESS_INDICATING_ETH).turnNum,
      finalizesAt: 1,
      // stateHash will be set to HashZero by this helper fn
      // if state property of this object is undefined
      state: lChannelAfter.someState(MAGIC_ADDRESS_INDICATING_ETH),
      outcome: lChannelAfter.outcome(MAGIC_ADDRESS_INDICATING_ETH),
    });

    expect(await testNitroAdjudicator.statusOf(sourceId)).toEqual(expectedStatusAfter);

    // assert that virtual channel did not change.

    expect(await testNitroAdjudicator.statusOf(targetId)).toEqual(
      channelDataToStatus({
        turnNumRecord: 99,
        finalizesAt: 1,
        outcome: vOutcome,
      })
    );
  });
});
