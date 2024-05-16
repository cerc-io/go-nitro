package chainservice

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	NitroAdjudicator "github.com/statechannels/go-nitro/node/engine/chainservice/adjudicator"
)

// assetAddressForIndex uses the input parameters of a transaction to map an asset index to an asset address
func assetAddressForIndex(na *NitroAdjudicator.NitroAdjudicator, tx *types.Transaction, index *big.Int) (common.Address, error) {
	abi, err := NitroAdjudicator.NitroAdjudicatorMetaData.GetAbi()
	if err != nil {
		return common.Address{}, err
	}
	params, err := decodeTxParams(abi, tx.Data())
	if err != nil {
		return common.Address{}, err
	}
	// TODO remove the assumption that the tx incudes a candidate parameter
	// 	concludeAndTransferAllAssets includes this parameter, but transferAllAssets, transfer, and claim do not.
	//  https://github.com/statechannels/go-nitro/issues/759
	value, exists := params["candidate"]
	if !exists || value == nil {
			return common.Address{}, nil
	}

	candidate := params["candidate"].(struct {
		VariablePart struct {
			Outcome []struct {
				Asset         common.Address "json:\"asset\""
				AssetMetadata struct {
					AssetType uint8   "json:\"assetType\""
					Metadata  []uint8 "json:\"metadata\""
				} "json:\"assetMetadata\""
				Allocations []struct {
					Destination    [32]uint8 "json:\"destination\""
					Amount         *big.Int  "json:\"amount\""
					AllocationType uint8     "json:\"allocationType\""
					Metadata       []uint8   "json:\"metadata\""
				} "json:\"allocations\""
			} "json:\"outcome\""
			AppData []uint8  "json:\"appData\""
			TurnNum *big.Int "json:\"turnNum\""
			IsFinal bool     "json:\"isFinal\""
		} "json:\"variablePart\""
		Sigs []struct {
			V uint8     "json:\"v\""
			R [32]uint8 "json:\"r\""
			S [32]uint8 "json:\"s\""
		} "json:\"sigs\""
	})
	return candidate.VariablePart.Outcome[index.Int64()].Asset, nil
}

func decodeTxParams(abi *abi.ABI, data []byte) (map[string]interface{}, error) {
	m, err := abi.MethodById(data[:4])
	if err != nil {
		return map[string]interface{}{}, err
	}
	v := map[string]interface{}{}
	if err := m.Inputs.UnpackIntoMap(v, data[4:]); err != nil {
		return map[string]interface{}{}, err
	}

	return v, nil
}
