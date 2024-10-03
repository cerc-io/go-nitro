package payments

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/statechannels/go-nitro/abi"
	"github.com/statechannels/go-nitro/channel/state"
	"github.com/statechannels/go-nitro/internal/queue"
	"github.com/statechannels/go-nitro/types"

	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	nc "github.com/statechannels/go-nitro/crypto"
)

const MAX_SWAP_STORAGE_LIMIT = 5

type Swap struct {
	Id        types.Destination
	ChannelId types.Destination
	Exchange  Exchange
	Sigs      map[uint]state.Signature // keyed by participant index in swap channel
	Nonce     uint64
}

func NewSwap(channelId types.Destination, tokenIn, tokenOut common.Address, amountIn, amountOut *big.Int, nonce uint64) Swap {
	swap := Swap{
		ChannelId: channelId,
		Exchange: Exchange{
			tokenIn,
			tokenOut,
			amountIn,
			amountOut,
		},
		Sigs:  make(map[uint]state.Signature, 2),
		Nonce: nonce,
	}
	swap.Id = swap.SwapId()

	return swap
}

// encodes the state into a []bytes value
func (s Swap) encode() (types.Bytes, error) {
	return ethAbi.Arguments{
		{Type: abi.Destination}, // channel id
		{Type: abi.Address},     // tokenIn
		{Type: abi.Address},     // tokenOut
		{Type: abi.Uint256},     // amountIn
		{Type: abi.Uint256},     // amountOut
		{Type: abi.Uint256},     // nonce
	}.Pack(
		s.ChannelId,
		s.Exchange.TokenIn,
		s.Exchange.TokenOut,
		s.Exchange.AmountIn,
		s.Exchange.AmountOut,
		new(big.Int).SetUint64(s.Nonce),
	)
}

func (s Swap) Equal(target Swap) bool {
	return s.ChannelId == target.ChannelId && s.Exchange.Equal(target.Exchange) && s.Nonce == target.Nonce
}

func (s Swap) Clone() Swap {
	clonedSigs := make(map[uint]state.Signature, len(s.Sigs))
	for i, sig := range s.Sigs {
		clonedSigs[i] = sig
	}

	return Swap{
		ChannelId: s.ChannelId,
		Exchange: Exchange{
			TokenIn:   s.Exchange.TokenIn,
			TokenOut:  s.Exchange.TokenOut,
			AmountIn:  s.Exchange.AmountIn,
			AmountOut: s.Exchange.AmountOut,
		},
		Sigs:  clonedSigs,
		Nonce: s.Nonce,
		Id:    s.Id,
	}
}

func (s Swap) SwapId() types.Destination {
	swapHash, err := s.Hash()
	if err != nil {
		return types.Destination{}
	}

	return types.Destination(swapHash)
}

// Hash returns the keccak256 hash of the State
func (sp Swap) Hash() (types.Bytes32, error) {
	encoded, err := sp.encode()
	if err != nil {
		return types.Bytes32{}, fmt.Errorf("failed to encode swap: %w", err)
	}

	return crypto.Keccak256Hash(encoded), nil
}

// Sign generates an ECDSA signature on the swap using the supplied private key
func (s Swap) Sign(secretKey []byte) (state.Signature, error) {
	hash, error := s.Hash()
	if error != nil {
		return state.Signature{}, error
	}

	return nc.SignEthereumMessage(hash.Bytes(), secretKey)
}

func (s Swap) AddSignature(sig state.Signature, myIndex uint) error {
	s.Sigs[myIndex] = sig

	return nil
}

// RecoverSigner computes the Ethereum address which generated Signature sig on Swap
func (s Swap) RecoverSigner(sig state.Signature) (types.Address, error) {
	hash, error := s.Hash()
	if error != nil {
		return types.Address{}, error
	}

	return nc.RecoverEthereumMessageSigner(hash[:], sig)
}

type jsonSwap struct {
	Id        types.Destination
	ChannelId types.Destination
	Exchange  Exchange
	Sigs      map[uint]state.Signature // keyed by participant index in swap channel
	Nonce     uint64
}

func (s *Swap) MarshalJSON() ([]byte, error) {
	jsonSwap := jsonSwap{
		Id:        s.Id,
		ChannelId: s.ChannelId,
		Exchange:  s.Exchange,
		Sigs:      s.Sigs,
		Nonce:     s.Nonce,
	}

	return json.Marshal(jsonSwap)
}

func (s *Swap) UnmarshalJSON(data []byte) error {
	var jsonSwap jsonSwap
	err := json.Unmarshal(data, &jsonSwap)
	if err != nil {
		return fmt.Errorf("error unmarshaling swap: %w", err)
	}

	s.Id = jsonSwap.Id
	s.ChannelId = jsonSwap.ChannelId
	s.Exchange = jsonSwap.Exchange
	s.Sigs = jsonSwap.Sigs
	s.Nonce = jsonSwap.Nonce

	return nil
}

type Exchange struct {
	TokenIn   common.Address
	TokenOut  common.Address
	AmountIn  *big.Int
	AmountOut *big.Int
}

func (ex Exchange) Equal(target Exchange) bool {
	return ex.TokenIn == target.TokenIn && ex.TokenOut == target.TokenOut && ex.AmountIn.Cmp(target.AmountIn) == 0 && ex.AmountOut.Cmp(target.AmountOut) == 0
}

// TODO: Store swap Id in fixed queue instead of swap
type SwapsQueue struct {
	queue.FixedQueue[Swap]
}

func NewSwapsQueue() *SwapsQueue {
	fixedQueue := queue.NewFixedQueue[Swap](MAX_SWAP_STORAGE_LIMIT)
	return &SwapsQueue{
		*fixedQueue,
	}
}

func (q *SwapsQueue) MarshalJSON() ([]byte, error) {
	var swapsIds []types.Destination
	swaps := q.Values()
	for _, swap := range swaps {
		swapsIds = append(swapsIds, swap.Id)
	}

	return json.Marshal(swapsIds)
}

func (q *SwapsQueue) UnmarshalJSON(data []byte) error {
	var swapsIds []types.Destination
	err := json.Unmarshal(data, &swapsIds)
	if err != nil {
		return err
	}

	for _, sId := range swapsIds {
		swap := Swap{Id: sId}
		q.Enqueue(swap)
	}

	return nil
}
