package types

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"sort"
	"sync/atomic"
	"time"

	eth2apiv1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	spectypes "github.com/ssvlabs/ssv-spec/types"

	beaconprotocol "github.com/ssvlabs/ssv/protocol/v2/blockchain/beacon"
)

const (
	MaxPossibleShareSize = 1326
	MaxAllowedShareSize  = MaxPossibleShareSize * 8 // Leaving some room for protocol updates and calculation mistakes.
)

// SSVShare is a combination of spectypes.Share and its Metadata.
type SSVShare struct {
	spectypes.Share
	Metadata

	committeeID atomic.Pointer[spectypes.CommitteeID]

	// minParticipationEpoch is the epoch at which the validator can start participating.
	// This is set on registration and on every reactivation.
	//
	// TODO: this is not persistent yet, so we should assume zero values are already participating for now.
	minParticipationEpoch phase0.Epoch
}

// BelongsToOperator checks whether the share belongs to operator.
func (s *SSVShare) BelongsToOperator(operatorID spectypes.OperatorID) bool {
	if operatorID == 0 {
		return false
	}

	return slices.ContainsFunc(s.Committee, func(shareMember *spectypes.ShareMember) bool {
		return shareMember.Signer == operatorID
	})
}

// HasBeaconMetadata checks whether the BeaconMetadata field is not nil.
func (s *SSVShare) HasBeaconMetadata() bool {
	return s != nil && s.BeaconMetadata != nil
}

func (s *SSVShare) IsAttesting(epoch phase0.Epoch) bool {
	return s.HasBeaconMetadata() &&
		(s.BeaconMetadata.IsAttesting() || (s.BeaconMetadata.Status == eth2apiv1.ValidatorStatePendingQueued && s.BeaconMetadata.ActivationEpoch <= epoch))
}

func (s *SSVShare) IsParticipating(epoch phase0.Epoch) bool {
	return !s.Liquidated && s.IsAttesting(epoch)
}

func (s *SSVShare) SetMinParticipationEpoch(epoch phase0.Epoch) {
	s.minParticipationEpoch = epoch
}

func (s *SSVShare) MinParticipationEpoch() phase0.Epoch {
	return s.minParticipationEpoch
}

func (s *SSVShare) SetFeeRecipient(feeRecipient bellatrix.ExecutionAddress) {
	s.FeeRecipientAddress = feeRecipient
}

// CommitteeID safely retrieves or computes the CommitteeID.
func (s *SSVShare) CommitteeID() spectypes.CommitteeID {
	// Load the current value of committeeID atomically.
	if ptr := s.committeeID.Load(); ptr != nil {
		return *ptr
	}

	// Compute the CommitteeID since it's not yet set.
	ids := make([]spectypes.OperatorID, len(s.Share.Committee))
	for i, v := range s.Share.Committee {
		ids[i] = v.Signer
	}
	id := ComputeCommitteeID(ids)

	// Create a new pointer and store it atomically.
	s.committeeID.Store(&id)
	return id
}

func (s *SSVShare) OperatorIDs() []spectypes.OperatorID {
	ids := make([]spectypes.OperatorID, len(s.Committee))
	for i, v := range s.Committee {
		ids[i] = v.Signer
	}
	return ids
}

func (s *SSVShare) HasQuorum(cnt uint64) bool {
	return cnt >= s.Quorum()
}

func (s *SSVShare) Quorum() uint64 {
	q, _ := ComputeQuorumAndPartialQuorum(uint64(len(s.Committee)))
	return q
}

// ComputeClusterIDHash will compute cluster ID hash with given owner address and operator ids
func ComputeClusterIDHash(address common.Address, operatorIds []uint64) []byte {
	sort.Slice(operatorIds, func(i, j int) bool {
		return operatorIds[i] < operatorIds[j]
	})

	// Encode the address and operator IDs in the same way as Solidity's abi.encodePacked
	var data []byte
	data = append(data, address.Bytes()...) // Address is 20 bytes
	for _, id := range operatorIds {
		idBytes := make([]byte, 32)                  // Each ID should be 32 bytes
		binary.BigEndian.PutUint64(idBytes[24:], id) // PutUint64 fills the last 8 bytes; rest are 0
		data = append(data, idBytes...)
	}

	// Hash the data using keccak256
	hash := crypto.Keccak256(data)
	return hash
}

func ComputeQuorumAndPartialQuorum(committeeSize uint64) (quorum uint64, partialQuorum uint64) {
	f := ComputeF(committeeSize)
	return f*2 + 1, f + 1
}

func ComputeF(committeeSize uint64) uint64 {
	return (committeeSize - 1) / 3
}

func ValidCommitteeSize(committeeSize uint64) bool {
	f := ComputeF(committeeSize)
	return (committeeSize-1)%3 == 0 && f >= 1 && f <= 4
}

// Metadata represents metadata of SSVShare.
type Metadata struct {
	BeaconMetadata *beaconprotocol.ValidatorMetadata
	OwnerAddress   common.Address
	Liquidated     bool

	// lastUpdated is an internal field that can be used to track the last time the metadata was updated.
	lastUpdated time.Time
}

func (m *Metadata) MetadataLastUpdated() time.Time {
	return m.lastUpdated
}

func (m *Metadata) SetMetadataLastUpdated(t time.Time) {
	m.lastUpdated = t
}

// Return a 32 bytes ID for the committee of operators
func ComputeCommitteeID(committee []spectypes.OperatorID) spectypes.CommitteeID {
	// sort
	sort.Slice(committee, func(i, j int) bool {
		return committee[i] < committee[j]
	})
	// Convert to bytes
	bytes := make([]byte, len(committee)*4)
	for i, v := range committee {
		binary.LittleEndian.PutUint32(bytes[i*4:], uint32(v)) // #nosec G115
	}
	// Hash
	return sha256.Sum256(bytes)
}
