// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package dpor

import (
	"encoding/json"
	"errors"

	"bitbucket.org/cpchain/chain/common"
	"bitbucket.org/cpchain/chain/consensus/dpor/election"
	"bitbucket.org/cpchain/chain/consensus/dpor/rpt"
	"bitbucket.org/cpchain/chain/core/types"
	"bitbucket.org/cpchain/chain/ethdb"
	"bitbucket.org/cpchain/chain/params"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

const (
	// EpochGapBetweenElectionAndMining is the the epoch gap between election and mining.
	EpochGapBetweenElectionAndMining = 1
	// MaxSizeOfRecentSigners is the size of the RecentSigners.
	MaxSizeOfRecentSigners = 10
)

var (
	errSignerNotInCommittee = errors.New("signer not in committee")
	errGenesisBlockNumber   = errors.New("Genesis block has no leader")
)

type Snapshot interface {
	store(db ethdb.Database) error

	//clone(snapshot *Snapshot) error
	//applySnapshot(headers []*types.Header, snapshot *Snapshot) error

	copy() *Snapshot
	apply(headers []*types.Header) (*Snapshot, error)
	applyHeader(header *types.Header) error
	updateCandidates(header *types.Header) error
	updateRpts(header *types.Header) (rpt.RPTs, error)
	updateView(rpts rpt.RPTs, seed int64, viewLength int) error
	signers() []common.Address
	signerRound(signer common.Address) (int, error)
	isSigner(signer common.Address) bool
	isLeader(signer common.Address, number uint64) (bool, error)
	candidates() []common.Address
	inturn(number uint64, signer common.Address) bool
}

// DporSnapshot is the state of the authorization voting at a given point in time.
type DporSnapshot struct {
	config   *params.DporConfig // Consensus engine parameters to fine tune behavior
	sigcache *lru.ARCCache      // Cache of recent block signatures to speed up ecrecover

	Number uint64      `json:"number"` // Block number where the Snapshot was created
	Hash   common.Hash `json:"hash"`   // Block hash where the Snapshot was created

	Candidates    []common.Address            `json:"candidates"` // Set of candidates read from campaign contract
	RecentSigners map[uint64][]common.Address `json:"signers"`    // Set of recent signers
}

// newSnapshot creates a new Snapshot with the specified startup parameters. This
// method does not initialize the set of recent signers, so only ever use if for
// the genesis block.
func newSnapshot(config *params.DporConfig, sigcache *lru.ARCCache, number uint64, hash common.Hash, signers []common.Address) *DporSnapshot {
	snap := &DporSnapshot{
		config:        config,
		sigcache:      sigcache,
		Number:        number,
		Hash:          hash,
		RecentSigners: make(map[uint64][]common.Address),
	}
	snap.RecentSigners[snap.EpochIdx()] = signers

	return snap
}

// loadSnapshot loads an existing Snapshot from the database.
func loadSnapshot(config *params.DporConfig, sigcache *lru.ARCCache, db ethdb.Database, hash common.Hash) (*DporSnapshot, error) {
	blob, err := db.Get(append([]byte("dpor-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(DporSnapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config
	snap.sigcache = sigcache

	return snap, nil
}

// store inserts the Snapshot into the database.
func (s *DporSnapshot) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("dpor-"), s.Hash[:]...), blob)
}

// copy creates a deep copy of the Snapshot, though not the individual votes.
func (s *DporSnapshot) copy() *DporSnapshot {
	cpy := &DporSnapshot{
		config:        s.config,
		sigcache:      s.sigcache,
		Number:        s.Number,
		Hash:          s.Hash,
		Candidates:    make([]common.Address, len(s.Candidates)),
		RecentSigners: make(map[uint64][]common.Address),
	}
	copy(cpy.Candidates, s.Candidates)
	for epochIdx, committee := range s.RecentSigners {
		cpy.RecentSigners[epochIdx] = committee
	}
	return cpy
}

// apply creates a new authorization Snapshot by applying the given headers to
// the original one.
func (s *DporSnapshot) apply(headers []*types.Header) (*DporSnapshot, error) {
	// Allow passing in no headers for cleaner code
	if len(headers) == 0 {
		return s, nil
	}
	// Sanity check that the headers can be applied
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, errInvalidVotingChain
		}
	}

	if headers[0].Number.Uint64() != s.Number+1 {
		return nil, errInvalidVotingChain
	}
	// Iterate through the headers and create a new Snapshot
	snap := s.copy()

	for _, header := range headers {
		err := snap.applyHeader(header)
		if err != nil {
			log.Warn("DporSnapshot apply header error.", err)
			return nil, err
		}
	}

	snap.Number = headers[len(headers)-1].Number.Uint64()
	snap.Hash = headers[len(headers)-1].Hash()

	return snap, nil
}

// TODO: finish this func to apply header to Snapshot to calculate reputations of candidates fetched from candidate contract.
func (s *DporSnapshot) applyHeader(header *types.Header) error {
	// update Snapshot attributes.
	s.Number = header.Number.Uint64()
	s.Hash = header.Hash()

	s.updateCandidates(header)

	if s.Number%checkpointInterval == 0 {
		rpts, err := s.updateRpts(header)
		if err != nil {
			return err
		}

		seed := header.Hash().Big().Int64()
		viewLength := int(s.config.Epoch)
		s.updateView(rpts, seed, viewLength)
	}
	return nil
}

// TODO: fix this logic.
func (s *DporSnapshot) updateCandidates(header *types.Header) error {
	// TODO: delete this.
	candidates := []common.Address{
		common.HexToAddress("0xe94b7b6c5a0e526a4d97f9768ad6097bde25c62a"),
		common.HexToAddress("0xc05302acebd0730e3a18a058d7d1cb1204c4a092"),
		common.HexToAddress("0xef3dd127de235f15ffb4fc0d71469d1339df6465"),
	}
	// TODO: above is wrong.

	s.Candidates = candidates
	return nil
}

// TODO: implement this func to get rpts for candidates. maybe save it as a map.
func (s *DporSnapshot) updateRpts(header *types.Header) (rpt.RPTs, error) {

	// TODO: fix this.
	/*
		collector := rpt.BasicCollector{}
		rpts := collector.GetRpts(&candidates, header.Number.Uint64())
	*/

	rpts := rpt.RPTs{
		rpt.RPT{
			Address: common.HexToAddress("0xe94b7b6c5a0e526a4d97f9768ad6097bde25c62a"),
			Rpt:     50,
		},
		rpt.RPT{
			Address: common.HexToAddress("0xc05302acebd0730e3a18a058d7d1cb1204c4a092"),
			Rpt:     100,
		},
		rpt.RPT{
			Address: common.HexToAddress("0xef3dd127de235f15ffb4fc0d71469d1339df6465"),
			Rpt:     20,
		},
		rpt.RPT{
			Address: common.HexToAddress("0x6E31e5B68A98dcD17264bd1ba547D0B3E874dA1E"),
			Rpt:     1000,
		},
		// rpt.RPT{
		// 	Address: common.HexToAddress("0x3a18598184ef84198db90c28fdfdfdf56544f747"),
		// 	Rpt:     300,
		// },
	}
	// TODO: above is wrong.

	return rpts, nil
}

// updateView use rpt and election result to get new committee(signers).
func (s *DporSnapshot) updateView(rpts rpt.RPTs, seed int64, viewLength int) error {
	signers := election.Elect(rpts, seed, viewLength)

	// TODO: fix this.

	epochIdx := s.EpochIdx() + EpochGapBetweenElectionAndMining
	s.RecentSigners[epochIdx] = signers

	if uint(len(s.RecentSigners)) > MaxSizeOfRecentSigners {
		delete(s.RecentSigners, s.EpochIdx()+EpochGapBetweenElectionAndMining-uint64(MaxSizeOfRecentSigners))
	}

	return nil
}

// EpochIdx returns the epoch index of current block number.
func (s *DporSnapshot) EpochIdx() uint64 {
	if s.Number == 0 {
		return 0
	}

	return (s.Number - 1) / uint64(epochLength)
}

// EpochIdxOf returns the epoch index of given block number.
func (s *DporSnapshot) EpochIdxOf(blockNum uint64) uint64 {
	if blockNum == 0 {
		return 0
	}

	return (blockNum - 1) / uint64(epochLength)
}

// Signers retrieves all signersOf in the committee.
func (s *DporSnapshot) signersOf(number uint64) []common.Address {
	return s.RecentSigners[s.EpochIdxOf(number)]
}

func (s *DporSnapshot) signerRoundOf(signer common.Address, number uint64) (int, error) {
	for round, s := range s.signersOf(number) {
		if s == signer {
			return round, nil
		}
	}
	return -1, errSignerNotInCommittee
}

func (s *DporSnapshot) isSigner(signer common.Address, number uint64) bool {
	_, err := s.signerRoundOf(signer, number)
	return err == nil
}

func (s *DporSnapshot) isLeader(signer common.Address, number uint64) (bool, error) {
	if number == 0 {
		return false, errGenesisBlockNumber
	}
	round, err := s.signerRoundOf(signer, number)
	if err != nil {
		return false, err
	}
	return round == int((number-1)%s.config.Epoch), nil
}

// Candidates retrieves all candidates recorded in the campaign contract.
func (s *DporSnapshot) candidates() []common.Address {
	return s.Candidates
}

// inturn returns if a signer at a given block height is in-turn or not.
func (s *DporSnapshot) inturn(number uint64, signer common.Address) bool {
	ok, err := s.isLeader(signer, number)
	if err != nil {
		return false
	}
	return ok
}

func (s *DporSnapshot) isFutureSigner(signer common.Address, number uint64) bool {
	for _, sn := range s.RecentSigners[s.EpochIdxOf(number)+EpochGapBetweenElectionAndMining] {
		if sn == signer {
			return true
		}
	}
	return false
}
