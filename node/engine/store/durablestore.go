package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/channel"
	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/crypto"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/protocols/bridgeddefund"
	"github.com/statechannels/go-nitro/protocols/bridgedfund"
	"github.com/statechannels/go-nitro/protocols/directdefund"
	"github.com/statechannels/go-nitro/protocols/directfund"
	"github.com/statechannels/go-nitro/protocols/mirrorbridgeddefund"
	"github.com/statechannels/go-nitro/protocols/swap"
	"github.com/statechannels/go-nitro/protocols/swapdefund"
	"github.com/statechannels/go-nitro/protocols/swapfund"
	"github.com/statechannels/go-nitro/protocols/virtualdefund"
	"github.com/statechannels/go-nitro/protocols/virtualfund"
	"github.com/statechannels/go-nitro/types"
	"github.com/tidwall/buntdb"
)

type DurableStore struct {
	objectives         *buntdb.DB
	channels           *buntdb.DB
	consensusChannels  *buntdb.DB
	channelToObjective *buntdb.DB
	vouchers           *buntdb.DB
	lastBlockNumSeen   *buntdb.DB
	swaps              *buntdb.DB
	channelToSwaps     *buntdb.DB

	key     string // the signing key of the store's engine
	address string // the (Ethereum) address associated to the signing key
	folder  string // the folder where the store's data is stored
}

// NewDurableStore creates a new DurableStore that uses the given folder to store its data
// It will create the folder if it does not exist
func NewDurableStore(key []byte, folder string, config buntdb.Config) (Store, error) {
	ps := DurableStore{}

	me := crypto.GetAddressFromSecretKeyBytes(key)
	dataFolder := filepath.Join(folder, me.String())

	err := os.MkdirAll(dataFolder, os.ModePerm)
	if err != nil {
		return nil, err
	}

	ps.key = common.Bytes2Hex(key)
	ps.address = crypto.GetAddressFromSecretKeyBytes(key).String()
	ps.folder = folder

	ps.objectives, err = ps.openDB("objectives", config)
	if err != nil {
		return nil, err
	}
	ps.channels, err = ps.openDB("channels", config)
	if err != nil {
		return nil, err
	}
	ps.consensusChannels, err = ps.openDB("consensus_channels", config)
	if err != nil {
		return nil, err
	}
	ps.channelToObjective, err = ps.openDB("channel_to_objective", config)
	if err != nil {
		return nil, err
	}
	ps.vouchers, err = ps.openDB("vouchers", config)
	if err != nil {
		return nil, err
	}

	ps.lastBlockNumSeen, err = ps.openDB("lastBlockNumSeen", config)
	if err != nil {
		return nil, err
	}

	ps.swaps, err = ps.openDB("swap", config)
	if err != nil {
		return nil, err
	}

	ps.channelToSwaps, err = ps.openDB("channelToSwaps", config)
	if err != nil {
		return nil, err
	}

	return &ps, nil
}

func (ds *DurableStore) openDB(name string, config buntdb.Config) (*buntdb.DB, error) {
	db, err := buntdb.Open(fmt.Sprintf("%s/%s_%s.db", ds.folder, name, ds.address[2:7]))
	if err != nil {
		return nil, err
	}
	err = db.SetConfig(config)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (ds *DurableStore) Close() error {
	err := ds.channels.Close()
	if err != nil {
		return err
	}
	err = ds.objectives.Close()
	if err != nil {
		return err
	}
	err = ds.consensusChannels.Close()
	if err != nil {
		return err
	}
	err = ds.channelToObjective.Close()
	if err != nil {
		return err
	}
	return ds.vouchers.Close()
}

func (ds *DurableStore) GetAddress() *types.Address {
	address := common.HexToAddress(ds.address)
	return &address
}

func (ds *DurableStore) GetChannelSecretKey() *[]byte {
	val := common.Hex2Bytes(ds.key)
	return &val
}

func (ds *DurableStore) GetSwapById(id types.Destination) (payments.Swap, error) {
	var sJSON string
	err := ds.swaps.View(func(tx *buntdb.Tx) error {
		var err error
		sJSON, err = tx.Get(id.String())
		return err
	})

	if errors.Is(err, buntdb.ErrNotFound) {
		return payments.Swap{}, ErrNoSuchSwap
	}
	var swap payments.Swap
	err = json.Unmarshal([]byte(sJSON), &swap)
	if err != nil {
		return payments.Swap{}, fmt.Errorf("error unmarshaling swap %s", id)
	}

	return swap, nil
}

func (ds *DurableStore) SetSwap(swap payments.Swap) error {
	sJSON, err := json.Marshal(swap)
	if err != nil {
		return err
	}

	err = ds.swaps.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(swap.Id.String(), string(sJSON), nil)
		return err
	})
	return err
}

func (ds *DurableStore) DestroySwapById(id types.Destination) error {
	return ds.swaps.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(id.String())
		return err
	})
}

func (ds *DurableStore) GetObjectiveById(id protocols.ObjectiveId) (protocols.Objective, error) {
	var obj protocols.Objective
	err := ds.objectives.View(func(tx *buntdb.Tx) error {
		objJSON, err := tx.Get(string(id))
		if err != nil {
			return err
		}

		obj, err = decodeObjective(id, []byte(objJSON))
		if err != nil {
			return fmt.Errorf("error decoding objective %s: %w", id, err)
		}

		err = ds.populateChannelData(obj)
		if err != nil {
			// return existing objective data along with error
			return fmt.Errorf("error populating channel data for objective %s: %w", id, err)
		}
		return nil
	})
	if err != nil && errors.Is(err, buntdb.ErrNotFound) {
		return nil, ErrNoSuchObjective
	}

	return obj, nil
}

func (ds *DurableStore) SetObjective(obj protocols.Objective) error {
	// todo: locking
	objJSON, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error setting objective %s: %w", obj.Id(), err)
	}

	err = ds.objectives.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(string(obj.Id()), string(objJSON), nil)
		return err
	})
	if err != nil {
		return err
	}
	for _, rel := range obj.Related() {
		switch related := rel.(type) {
		case *channel.VirtualChannel:
			err := ds.SetChannel(&related.Channel)
			if err != nil {
				return fmt.Errorf("error setting virtual channel %s from objective %s: %w", related.Id, obj.Id(), err)
			}
		case *channel.SwapChannel:
			err := ds.SetChannel(&related.Channel)
			if err != nil {
				return fmt.Errorf("error setting swap channel %s from objective %s: %w", related.Id, obj.Id(), err)
			}

		case *payments.Swap:
			err := ds.SetSwap(*related)
			if err != nil {
				return fmt.Errorf("error setting swap %s from objective %s: %w", related.Id, obj.Id(), err)
			}

			so, isSwapObj := obj.(*swap.Objective)
			if !isSwapObj {
				return fmt.Errorf("expected swap objective")
			}

			if so.GetStatus() == protocols.Completed && so.SwapStatus == types.Accepted {
				// Add swap to channelToSwaps map if successful
				removedSwap, err := ds.SetChannelToSwaps(*related)
				if err != nil {
					return fmt.Errorf("error setting channel to swaps %s from objective %s: %w", related.Id, obj.Id(), err)
				}

				// Remove old swap if exist
				if !removedSwap.Id.IsZero() {
					err = ds.DestroySwapById(removedSwap.Id)
					if err != nil {
						return fmt.Errorf("error in destroying old swap %s: %w", removedSwap.Id, err)
					}
				}
			}

			if so.GetStatus() == protocols.Rejected {
				// Delete the rejected swap
				err = ds.DestroySwapById(so.Swap.Id)
				if err != nil {
					return fmt.Errorf("error in destroying old swap %s: %w", so.Swap.Id, err)
				}
			}

		case *channel.Channel:
			err := ds.SetChannel(related)
			if err != nil {
				return fmt.Errorf("error setting channel %s from objective %s: %w", related.Id, obj.Id(), err)
			}
		case *consensus_channel.ConsensusChannel:
			err := ds.SetConsensusChannel(related)
			if err != nil {
				return fmt.Errorf("error setting consensus channel %s from objective %s: %w", related.Id, obj.Id(), err)
			}
		default:
			return fmt.Errorf("unexpected type: %T", rel)
		}
	}

	// Objective ownership can only be transferred if the channel is not owned by another objective
	var prevOwner protocols.ObjectiveId
	var isOwned bool = false
	err = ds.channelToObjective.View(func(tx *buntdb.Tx) error {
		res, err := tx.Get(string(obj.OwnsChannel().String()))
		if err != nil {
			return nil
		}
		prevOwner = protocols.ObjectiveId(res)
		isOwned = true
		return nil
	})
	if err != nil {
		return err
	}

	if status := obj.GetStatus(); status == protocols.Approved && !obj.OwnsChannel().IsZero() {
		if !isOwned {
			err := ds.channelToObjective.Update(func(tx *buntdb.Tx) error {
				_, _, err := tx.Set(string(obj.OwnsChannel().String()), string(obj.Id()), nil)
				return err
			})
			if err != nil {
				return fmt.Errorf("cannot transfer ownership of channel: %w", err)
			}

		}
		if isOwned && prevOwner != obj.Id() {
			return fmt.Errorf("cannot transfer ownership of channel from objective %s to %s", prevOwner, obj.Id())
		}
	}

	return nil
}

// GetLastBlockNumSeen retrieves the last blockchain block processed by this node
func (ds *DurableStore) GetLastBlockNumSeen() (uint64, error) {
	var result uint64
	err := ds.lastBlockNumSeen.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(lastBlockNumSeenKey)
		if err != nil {
			if errors.Is(err, buntdb.ErrNotFound) {
				result = 0
				return nil
			}
			return err
		}
		result, err = strconv.ParseUint(val, 10, 64)
		return err
	})
	return result, err
}

// SetLastBlockNumSeen sets the last blockchain block processed by this node
func (ds *DurableStore) SetLastBlockNumSeen(blockNumber uint64) error {
	return ds.lastBlockNumSeen.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(lastBlockNumSeenKey, strconv.FormatUint(blockNumber, 10), nil)
		return err
	})
}

// SetChannel sets the channel in the store.
func (ds *DurableStore) SetChannel(ch *channel.Channel) error {
	chJSON, err := ch.MarshalJSON()
	if err != nil {
		return err
	}

	err = ds.channels.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(ch.Id.String(), string(chJSON), nil)
		return err
	})
	return err
}

// DestroyChannel deletes the channel with id id.
func (ds *DurableStore) DestroyChannel(id types.Destination) error {
	return ds.channels.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(id.String())
		return err
	})
}

// SetConsensusChannel sets the channel in the store.
func (ps *DurableStore) SetConsensusChannel(ch *consensus_channel.ConsensusChannel) error {
	if ch.Id.IsZero() {
		return fmt.Errorf("cannot store a channel with a zero id")
	}
	chJSON, err := ch.MarshalJSON()
	if err != nil {
		return err
	}

	err = ps.consensusChannels.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(ch.Id.String(), string(chJSON), nil)
		return err
	})

	return err
}

// DestroyChannel deletes the channel with id id.
func (ds *DurableStore) DestroyConsensusChannel(id types.Destination) error {
	return ds.consensusChannels.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(id.String())
		return err
	})
}

// GetChannelById retrieves the channel with the supplied id, if it exists.
func (ds *DurableStore) GetChannelById(id types.Destination) (c *channel.Channel, ok bool) {
	ch, err := ds.getChannelById(id)
	if err != nil {
		return &channel.Channel{}, false
	}

	return &ch, true
}

// getChannelById returns the stored channel
func (ds *DurableStore) getChannelById(id types.Destination) (channel.Channel, error) {
	var chJSON string
	err := ds.channels.View(func(tx *buntdb.Tx) error {
		var err error
		chJSON, err = tx.Get(id.String())
		return err
	})

	if errors.Is(err, buntdb.ErrNotFound) {
		return channel.Channel{}, ErrNoSuchChannel
	}
	var ch channel.Channel
	err = ch.UnmarshalJSON([]byte(chJSON))
	if err != nil {
		return channel.Channel{}, fmt.Errorf("error unmarshaling channel %s", ch.Id)
	}

	return ch, nil
}

// GetChannelsByIds returns any channels with ids in the supplied list.
func (ds *DurableStore) GetChannelsByIds(ids []types.Destination) ([]*channel.Channel, error) {
	toReturn := []*channel.Channel{}
	// We know every channel has a unique id
	// so we can stop looking once we've found the correct number of channels

	var err error

	txError := ds.channels.View(func(tx *buntdb.Tx) error {
		return tx.Ascend("", func(key, chJSON string) bool {
			var ch channel.Channel
			err = json.Unmarshal([]byte(chJSON), &ch)
			if err != nil {
				return false
			}

			// If the channel is one of the ones we're looking for, add it to the list
			if contains(ids, ch.Id) {
				toReturn = append(toReturn, &ch)
			}

			// If we've found all the channels we need, stop looking
			if len(toReturn) == len(ids) {
				return false
			}
			return true // otherwise, continue looking
		})
	})

	if txError != nil {
		return []*channel.Channel{}, txError
	}
	if err != nil {
		return []*channel.Channel{}, err
	}

	return toReturn, nil
}

// GetChannelsByAppDefinition returns any channels that include the given app definition
func (ds *DurableStore) GetChannelsByAppDefinition(appDef types.Address) ([]*channel.Channel, error) {
	toReturn := []*channel.Channel{}
	var unmarshErr error
	err := ds.channels.View(func(tx *buntdb.Tx) error {
		return tx.Ascend("", func(key, chJSON string) bool {
			var ch channel.Channel
			unmarshErr = json.Unmarshal([]byte(chJSON), &ch)
			if unmarshErr != nil {
				return false
			}

			if ch.AppDefinition == appDef {
				toReturn = append(toReturn, &ch)
			}

			return true
		})
	})
	if err != nil {
		return []*channel.Channel{}, err
	}
	if unmarshErr != nil {
		return []*channel.Channel{}, unmarshErr
	}
	return toReturn, nil
}

// GetChannelsByParticipant returns any channels that include the given participant
func (ds *DurableStore) GetChannelsByParticipant(participant types.Address) ([]*channel.Channel, error) {
	toReturn := []*channel.Channel{}
	err := ds.channels.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("", func(key, chJSON string) bool {
			var ch channel.Channel
			err := json.Unmarshal([]byte(chJSON), &ch)
			if err != nil {
				return true // channel not found, continue looking
			}

			participants := ch.FixedPart.Participants
			for _, p := range participants {
				if p == participant {
					toReturn = append(toReturn, &ch)
				}
			}

			return true // channel not found: continue looking
		})
		return err
	})
	if err != nil {
		return []*channel.Channel{}, err
	}
	return toReturn, nil
}

func (ds *DurableStore) GetAllConsensusChannels() ([]*consensus_channel.ConsensusChannel, error) {
	toReturn := []*consensus_channel.ConsensusChannel{}
	var unmarshErr error
	err := ds.consensusChannels.View(func(tx *buntdb.Tx) error {
		return tx.Ascend("", func(key, chJSON string) bool {
			var ch consensus_channel.ConsensusChannel

			unmarshErr = json.Unmarshal([]byte(chJSON), &ch)
			if unmarshErr != nil {
				return false
			}
			toReturn = append(toReturn, &ch)
			return true
		})
	})
	if err != nil {
		return []*consensus_channel.ConsensusChannel{}, err
	}

	if unmarshErr != nil {
		return []*consensus_channel.ConsensusChannel{}, unmarshErr
	}
	return toReturn, nil
}

// GetAllChannels retrieves all channels stored in the DurableStore
func (ds *DurableStore) GetAllChannels() ([]*channel.Channel, error) {
	toReturn := []*channel.Channel{}
	var unmarshErr error
	err := ds.channels.View(func(tx *buntdb.Tx) error {
		return tx.Ascend("", func(key, chJSON string) bool {
			var ch channel.Channel

			unmarshErr = json.Unmarshal([]byte(chJSON), &ch)
			if unmarshErr != nil {
				return false
			}
			toReturn = append(toReturn, &ch)
			return true
		})
	})
	if err != nil {
		return []*channel.Channel{}, err
	}
	if unmarshErr != nil {
		return []*channel.Channel{}, unmarshErr
	}
	return toReturn, nil
}

// GetConsensusChannelById returns a ConsensusChannel with the given channel id
func (ds *DurableStore) GetConsensusChannelById(id types.Destination) (channel *consensus_channel.ConsensusChannel, err error) {
	var ch *consensus_channel.ConsensusChannel
	err = ds.consensusChannels.View(func(tx *buntdb.Tx) error {
		chJSON, err := tx.Get(id.String())

		if errors.Is(err, buntdb.ErrNotFound) {
			return ErrNoSuchChannel
		}

		ch = &consensus_channel.ConsensusChannel{}
		err = ch.UnmarshalJSON([]byte(chJSON))
		if err != nil {
			return fmt.Errorf("error unmarshaling channel %s", ch.Id)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// GetConsensusChannel returns a ConsensusChannel between the calling node and
// the supplied counterparty, if such channel exists
func (ps *DurableStore) GetConsensusChannel(counterparty types.Address) (channel *consensus_channel.ConsensusChannel, ok bool) {
	err := ps.consensusChannels.View(func(tx *buntdb.Tx) error {
		return tx.Ascend("", func(key, chJSON string) bool {
			var ch consensus_channel.ConsensusChannel
			err := json.Unmarshal([]byte(chJSON), &ch)
			if err != nil {
				return true // channel not found, continue looking
			}

			participants := ch.Participants()
			if len(participants) == 2 {
				if participants[0] == counterparty || participants[1] == counterparty {
					channel = &ch
					ok = true
					return false // we have found the target channel: break the Range loop
				}
			}

			return true // channel not found: continue looking
		})
	})
	if err != nil {
		return nil, false
	}
	return
}

func (ps *DurableStore) GetObjectiveByChannelId(channelId types.Destination) (protocols.Objective, bool) {
	var id protocols.ObjectiveId

	err := ps.channelToObjective.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(channelId.String())
		id = protocols.ObjectiveId(val)

		return err
	})
	if err != nil {
		return &directfund.Objective{}, false
	}

	objective, err := ps.GetObjectiveById(protocols.ObjectiveId(id))
	return objective, err == nil
}

// populateChannelData fetches stored Channel data relevant to the given
// objective and attaches it to the objective. The channel data is attached
// in-place of the objectives existing channel pointers.
func (ds *DurableStore) populateChannelData(obj protocols.Objective) error {
	id := obj.Id()

	switch o := obj.(type) {
	case *directfund.Objective:
		ch, err := ds.getChannelById(o.C.Id)
		if err != nil {
			return fmt.Errorf("error retrieving channel data for objective %s: %w", id, err)
		}

		o.C = &ch

		return nil
	case *directdefund.Objective:

		ch, err := ds.getChannelById(o.C.Id)
		if err != nil {
			return fmt.Errorf("error retrieving channel data for objective %s: %w", id, err)
		}

		o.C = &ch

		// Populate virtual channels if present
		if len(o.FundedChannels) != 0 {
			for virtualChannelId := range o.FundedChannels {
				updatedVirtualChannel, _ := ds.GetChannelById(virtualChannelId)
				o.FundedChannels[virtualChannelId] = updatedVirtualChannel
			}
		}

		return nil
	case *virtualfund.Objective:
		v, err := ds.getChannelById(o.V.Id)
		if err != nil {
			return fmt.Errorf("error retrieving virtual channel data for objective %s: %w", id, err)
		}
		o.V = &channel.VirtualChannel{Channel: v}

		zeroAddress := types.Destination{}

		if o.ToMyLeft != nil &&
			o.ToMyLeft.Channel != nil &&
			o.ToMyLeft.Channel.Id != zeroAddress {

			left, err := ds.GetConsensusChannelById(o.ToMyLeft.Channel.Id)
			if err != nil {
				return fmt.Errorf("error retrieving left ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyLeft.Channel = left
		}

		if o.ToMyRight != nil &&
			o.ToMyRight.Channel != nil &&
			o.ToMyRight.Channel.Id != zeroAddress {
			right, err := ds.GetConsensusChannelById(o.ToMyRight.Channel.Id)
			if err != nil {
				return fmt.Errorf("error retrieving right ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyRight.Channel = right
		}

		return nil
	case *swap.Objective:
		ch, err := ds.getChannelById(o.C.Id)
		if err != nil {
			return fmt.Errorf("error retrieving channel data for objective %s: %w", id, err)
		}

		swap, err := ds.GetSwapById(o.Swap.Id)
		if err != nil {
			return fmt.Errorf("error getting swap by Id: %w", err)
		}

		o.Swap = swap
		o.C = &channel.SwapChannel{Channel: ch}
		return nil
	case *swapfund.Objective:
		v, err := ds.getChannelById(o.S.Id)
		if err != nil {
			return fmt.Errorf("error retrieving swap channel data for objective %s: %w", id, err)
		}
		o.S = &channel.SwapChannel{Channel: v}

		zeroAddress := types.Destination{}

		if o.ToMyLeft != nil &&
			o.ToMyLeft.Channel != nil &&
			o.ToMyLeft.Channel.Id != zeroAddress {

			left, err := ds.GetConsensusChannelById(o.ToMyLeft.Channel.Id)
			if err != nil {
				return fmt.Errorf("error retrieving left ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyLeft.Channel = left
		}

		if o.ToMyRight != nil &&
			o.ToMyRight.Channel != nil &&
			o.ToMyRight.Channel.Id != zeroAddress {
			right, err := ds.GetConsensusChannelById(o.ToMyRight.Channel.Id)
			if err != nil {
				return fmt.Errorf("error retrieving right ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyRight.Channel = right
		}

		return nil
	case *virtualdefund.Objective:
		v, err := ds.getChannelById(o.V.Id)
		if err != nil {
			return fmt.Errorf("error retrieving virtual channel data for objective %s: %w", id, err)
		}
		o.V = &channel.VirtualChannel{Channel: v}

		zeroAddress := types.Destination{}

		if o.ToMyLeft != nil &&
			o.ToMyLeft.Id != zeroAddress {

			left, err := ds.GetConsensusChannelById(o.ToMyLeft.Id)
			if err != nil {
				return fmt.Errorf("error retrieving left ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyLeft = left
		}

		if o.ToMyRight != nil &&
			o.ToMyRight.Id != zeroAddress {
			right, err := ds.GetConsensusChannelById(o.ToMyRight.Id)
			if err != nil {
				return fmt.Errorf("error retrieving right ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyRight = right
		}
		return nil
	case *swapdefund.Objective:
		s, err := ds.getChannelById(o.S.Id)
		if err != nil {
			return fmt.Errorf("error retrieving virtual channel data for objective %s: %w", id, err)
		}
		o.S = &channel.SwapChannel{Channel: s}

		zeroAddress := types.Destination{}

		if o.ToMyLeft != nil &&
			o.ToMyLeft.Id != zeroAddress {

			left, err := ds.GetConsensusChannelById(o.ToMyLeft.Id)
			if err != nil {
				return fmt.Errorf("error retrieving left ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyLeft = left
		}

		if o.ToMyRight != nil &&
			o.ToMyRight.Id != zeroAddress {
			right, err := ds.GetConsensusChannelById(o.ToMyRight.Id)
			if err != nil {
				return fmt.Errorf("error retrieving right ledger channel data for objective %s: %w", id, err)
			}
			o.ToMyRight = right
		}
		return nil
	case *bridgedfund.Objective:
		ch, err := ds.getChannelById(o.C.Id)
		if err != nil {
			return fmt.Errorf("error retrieving channel data for objective %s: %w", id, err)
		}

		o.C = &ch

		return nil

	case *bridgeddefund.Objective:
		ch, err := ds.getChannelById(o.C.Id)
		if err != nil {
			return fmt.Errorf("error retrieving channel data for objective %s: %w", id, err)
		}

		o.C = &ch

		return nil

	case *mirrorbridgeddefund.Objective:
		ch, err := ds.getChannelById(o.C.Id)
		if err != nil {
			return fmt.Errorf("error retrieving channel data for objective %s: %w", id, err)
		}

		o.C = &ch

		return nil

	default:
		return fmt.Errorf("objective %s did not correctly represent a known Objective type", id)
	}
}

func (ds *DurableStore) ReleaseChannelFromOwnership(channelId types.Destination) error {
	return ds.channelToObjective.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(channelId.String())
		return err
	})
}

func (ds *DurableStore) SetVoucherInfo(channelId types.Destination, v payments.VoucherInfo) error {
	return ds.vouchers.Update(func(tx *buntdb.Tx) error {
		vJSON, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, _, err = tx.Set(channelId.String(), string(vJSON), nil)

		return err
	})
}

func (ds *DurableStore) GetVoucherInfo(channelId types.Destination) (*payments.VoucherInfo, error) {
	v := &payments.VoucherInfo{}
	err := ds.vouchers.View(func(tx *buntdb.Tx) error {
		vJSON, err := tx.Get(channelId.String())
		if err != nil {
			return fmt.Errorf("channelId %s: %w", channelId.String(), ErrLoadVouchers)
		}
		return json.Unmarshal([]byte(vJSON), v)
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (ds *DurableStore) RemoveVoucherInfo(channelId types.Destination) error {
	return ds.vouchers.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(channelId.String())
		return err
	})
}

func (ds *DurableStore) DestroyObjective(id protocols.ObjectiveId) error {
	return ds.objectives.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete(string(id))
		return err
	})
}

func (ds *DurableStore) GetPendingSwapByChannelId(id types.Destination) (*payments.Swap, error) {
	var pendingSwapId types.Destination
	err := ds.objectives.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("", func(key, objJSON string) bool {
			objId := protocols.ObjectiveId(key)
			if !swap.IsSwapObjective(objId) {
				return true // objective not found, continue looking
			}

			var obj swap.Objective
			err := json.Unmarshal([]byte(objJSON), &obj)
			if err != nil {
				return true // objective not found, continue looking
			}

			if obj.C.Id == id && obj.SwapStatus == types.PendingConfirmation {
				pendingSwapId = obj.Swap.Id
				return false // objective found, stop iteration
			}

			return true // objective not found: continue looking
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	swap, err := ds.GetSwapById(pendingSwapId)
	if err == nil {
		return &swap, nil
	}

	return nil, nil
}

func (ds *DurableStore) GetSwapsByChannelId(id types.Destination) ([]payments.Swap, error) {
	swapQueue := payments.NewSwapsQueue()

	err := ds.channelToSwaps.View(func(tx *buntdb.Tx) error {
		sJSON, err := tx.Get(id.String())

		if errors.Is(err, buntdb.ErrNotFound) {
			return nil
		}

		err = swapQueue.UnmarshalJSON([]byte(sJSON))
		if err != nil {
			return fmt.Errorf("error unmarshalling swap queue %w", err)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	var swapsToReturn []payments.Swap
	swaps := swapQueue.Values()
	for _, swap := range swaps {
		s, err := ds.GetSwapById(swap.Id)
		if errors.Is(err, ErrNoSuchSwap) {
			continue
		}
		swapsToReturn = append(swapsToReturn, s)
	}

	return swapsToReturn, nil
}

func (ds *DurableStore) SetChannelToSwaps(swap payments.Swap) (payments.Swap, error) {
	swapQueue := payments.NewSwapsQueue()

	err := ds.channelToSwaps.View(func(tx *buntdb.Tx) error {
		sJSON, err := tx.Get(swap.ChannelId.String())

		if errors.Is(err, buntdb.ErrNotFound) {
			return nil
		}

		err = swapQueue.UnmarshalJSON([]byte(sJSON))
		if err != nil {
			return fmt.Errorf("error unmarshalling swap queue %w", err)
		}

		return err
	})
	if err != nil {
		return payments.Swap{}, err
	}

	removedSwap := swapQueue.Enqueue(swap)

	swapsJson, err := swapQueue.MarshalJSON()
	if err != nil {
		return payments.Swap{}, fmt.Errorf("error marshalling swap queue %w", err)
	}

	err = ds.channelToSwaps.Update(func(tx *buntdb.Tx) error {
		_, _, err = tx.Set(swap.ChannelId.String(), string(swapsJson), nil)
		return err
	})
	if err != nil {
		return payments.Swap{}, err
	}

	return removedSwap, nil
}
