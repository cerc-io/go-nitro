package protocols

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/statechannels/go-nitro/channel/consensus_channel"
	"github.com/statechannels/go-nitro/payments"
	"github.com/statechannels/go-nitro/types"
)

// ObjectivePayload is a message containing a payload of []byte that an objective is responsible for decoding.
type ObjectivePayload struct {
	// PayloadData is the serialized json payload
	PayloadData []byte
	// ObjectiveId is the id of the objective that is responsible for decoding and handling the payload
	ObjectiveId ObjectiveId
	// Type is the type of the payload the message contains.
	// This is useful when a protocol wants to handle different types of payloads.
	Type PayloadType
}

type PayloadType string

// CreateObjectivePayload generates an objective message from the given objective id and payload.
// CreateObjectivePayload handles serializing `p` into json.
func CreateObjectivePayload(id ObjectiveId, payloadType PayloadType, p interface{}) (ObjectivePayload, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return ObjectivePayload{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return ObjectivePayload{PayloadData: b, ObjectiveId: id, Type: payloadType}, nil
}

// Message is an object to be sent across the wire.
type Message struct {
	To   types.Address
	From types.Address
	// ObjectivePayloads contains a collection of payloads for various objectives.
	// Protocols are responsible for parsing the payload.
	ObjectivePayloads []ObjectivePayload
	// LedgerProposals contains a collection of signed proposals.
	// Since proposals need to be handled in order they need to be an explicit part of the message format.
	LedgerProposals []consensus_channel.SignedProposal
	// Payments contains a collection of signed vouchers representing payments.
	// Payments are handled outside of any objective.
	Payments []payments.Voucher
	// RejectedObjectives is a collection of objectives that have been rejected.
	RejectedObjectives []ObjectiveId
}

// Serialize serializes the message into a string.
func (m Message) Serialize() (string, error) {
	bytes, err := json.Marshal(m)
	return string(bytes), err
}

// Merge accepts a SideEffects struct that is merged into the the existing SideEffects.
func (se *SideEffects) Merge(other SideEffects) {
	se.MessagesToSend = append(se.MessagesToSend, other.MessagesToSend...)
	se.TransactionsToSubmit = append(se.TransactionsToSubmit, other.TransactionsToSubmit...)
	se.ProposalsToProcess = append(se.ProposalsToProcess, other.ProposalsToProcess...)
}

// GetProposalObjectiveId returns the objectiveId for a proposal.
func GetProposalObjectiveId(p consensus_channel.Proposal, channelType types.ChannelType) (ObjectiveId, error) {
	switch p.Type() {
	case "AddProposal":
		{
			var prefix string
			if channelType == types.Swap {
				prefix = "SwapFund-"
			} else {
				prefix = "VirtualFund-"
			}
			channelId := p.ToAdd.Guarantee.Target().String()
			return ObjectiveId(prefix + channelId), nil

		}
	case "RemoveProposal":
		{
			var prefix string
			if channelType == types.Swap {
				prefix = "SwapDefund-"
			} else {
				prefix = "VirtualDefund-"
			}
			channelId := p.ToRemove.Target.String()
			return ObjectiveId(prefix + channelId), nil

		}
	default:
		{
			return "", errors.New("invalid proposal type")
		}
	}
}

// CreateObjectivePayloadMessage returns a message for each recipient tht contains an objective payload.
func CreateObjectivePayloadMessage(id ObjectiveId, p interface{}, payloadType PayloadType, recipients ...types.Address) ([]Message, error) {
	messages := make([]Message, 0)

	for _, participant := range recipients {
		payload, err := CreateObjectivePayload(id, payloadType, p)
		if err != nil {
			return []Message{}, err
		}
		message := Message{To: participant, ObjectivePayloads: []ObjectivePayload{payload}}
		messages = append(messages, message)
	}
	return messages, nil
}

// CreateSignedProposalMessage returns a signed proposal message addressed to the counterparty in the given ledger
// It contains the provided signed proposals and any proposals in the proposal queue.
func CreateRejectionNoticeMessage(oId ObjectiveId, recipients ...types.Address) []Message {
	messages := make([]Message, 0)
	for _, recipient := range recipients {
		message := Message{To: recipient, RejectedObjectives: []ObjectiveId{oId}}
		messages = append(messages, message)
	}

	return messages
}

// CreateSignedProposalMessage returns a signed proposal message addressed to the counterparty in the given ledger channel.
// The proposals MUST be sorted by turnNum
// since the ledger protocol relies on the message receipient processing the proposals in that order. See ADR 4.
func CreateSignedProposalMessage(recipient types.Address, proposals ...consensus_channel.SignedProposal) Message {
	msg := Message{To: recipient, LedgerProposals: proposals}
	return msg
}

// CreateVoucherMessage returns a signed voucher message for each of the recipients provided.
func CreateVoucherMessage(voucher payments.Voucher, recipients ...types.Address) []Message {
	messages := make([]Message, len(recipients))
	for i, recipient := range recipients {
		messages[i] = Message{To: recipient, Payments: []payments.Voucher{voucher}}
	}

	return messages
}

// DeserializeMessage deserializes the passed string into a protocols.Message.
func DeserializeMessage(s string) (Message, error) {
	msg := Message{}
	err := json.Unmarshal([]byte(s), &msg)

	return msg, err
}

// MessageSummary is a summary of a message suitable for logging.
type MessageSummary struct {
	To               string
	From             string
	PayloadSummaries []ObjectivePayloadSummary

	ProposalSummaries []ProposalSummary

	Payments []PaymentSummary
	// RejectedObjectives is a collection of objectives that have been rejected.
	RejectedObjectives []string
}

// ObjectivePayloadSummary is a summary of an objective payload suitable for logging.
type ObjectivePayloadSummary struct {
	ObjectiveId     string
	Type            string
	PayloadDataSize int
}

// ProposalSummary is a summary of a proposal suitable for logging.
type ProposalSummary struct {
	ObjectiveId  string
	LedgerId     string
	ProposalType string
	TurnNum      uint64
}

// PaymentSummary is a summary of a payment voucher suitable for logging.
type PaymentSummary struct {
	Amount    uint64
	ChannelId string
}

type getChannelByIdFn func(channelId types.Destination) (types.ChannelType, error)

// Summarize returns a MessageSummary for the message that is suitable for logging
func (m Message) Summarize(getChannelById getChannelByIdFn) MessageSummary {
	s := MessageSummary{}
	s.To = m.To.String()[0:8]
	s.From = m.From.String()[0:8]

	s.PayloadSummaries = make([]ObjectivePayloadSummary, len(m.ObjectivePayloads))
	for i, p := range m.ObjectivePayloads {
		s.PayloadSummaries[i] = ObjectivePayloadSummary{ObjectiveId: string(p.ObjectiveId), Type: string(p.Type), PayloadDataSize: len(p.PayloadData)}
	}

	s.ProposalSummaries = make([]ProposalSummary, len(m.LedgerProposals))
	for i, p := range m.LedgerProposals {
		var objIdString string

		t, err := getChannelById(p.Proposal.Target())
		if err != nil {
			objIdString = err.Error() // Use error message as objective id
		} else {
			objId, err := GetProposalObjectiveId(p.Proposal, t)
			objIdString = string(objId)
			if err != nil {
				objIdString = err.Error() // Use error message as objective id
			}
		}

		s.ProposalSummaries[i] = ProposalSummary{
			ObjectiveId:  objIdString,
			LedgerId:     p.ChannelID().String(),
			TurnNum:      p.TurnNum,
			ProposalType: string(p.Proposal.Type()),
		}
	}
	s.Payments = make([]PaymentSummary, len(m.Payments))
	for i, p := range m.Payments {
		s.Payments[i] = PaymentSummary{Amount: p.Amount.Uint64(), ChannelId: p.ChannelId.String()}
	}

	s.RejectedObjectives = make([]string, len(m.RejectedObjectives))
	for i, o := range m.RejectedObjectives {
		s.RejectedObjectives[i] = string(o)
	}
	return s
}

type Summary interface {
	ObjectivePayloadSummary | ProposalSummary | PaymentSummary | string
}
