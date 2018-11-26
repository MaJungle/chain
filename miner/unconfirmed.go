// Copyright 2016 The go-ethereum Authors

package miner

import (
	"container/ring"
	"sync"

	"bitbucket.org/cpchain/chain/commons/log"
	"bitbucket.org/cpchain/chain/types"
	"github.com/ethereum/go-ethereum/common"
)

// headerRetriever is used by the unconfirmed block set to verify whether a previously
// mined block is part of the canonical chain or not.
type headerRetriever interface {
	// GetHeaderByNumber retrieves the *canonical* header associated with a block number.
	GetHeaderByNumber(number uint64) *types.Header
}

// unconfirmedBlock is a small collection of metadata about a locally mined block
// that is placed into a unconfirmed set for canonical chain inclusion tracking.
type unconfirmedBlock struct {
	index uint64
	hash  common.Hash
}

// unconfirmedBlocks implements a data structure to maintain locally mined blocks
// have have not yet reached enough maturity to guarantee chain inclusion. It is
// used by the miner to provide logs to the user when a previously mined block
// has a high enough guarantee to not be reorged out of the canonical chain.
type unconfirmedBlocks struct {
	chain  headerRetriever // Blockchain to verify canonical status through
	depth  uint            // Depth after which to discard previous blocks
	blocks *ring.Ring      // Block infos to allow canonical chain cross checks
	lock   sync.RWMutex    // Protects the fields from concurrent access
}

// newUnconfirmedBlocks returns new data structure to track currently unconfirmed blocks.
// it doesn't store the block, only the number and the hash.
func newUnconfirmedBlocks(chain headerRetriever, depth uint) *unconfirmedBlocks {
	return &unconfirmedBlocks{
		chain: chain,
		depth: depth,
	}
}

// Insert adds a new block to the set of unconfirmed ones.
func (set *unconfirmedBlocks) Insert(index uint64, hash common.Hash) {
	// if a new block was mined locally, shift out any old enough blocks
	set.Shift(index)

	// create the new item as its own ring
	item := ring.New(1)
	item.Value = &unconfirmedBlock{
		index: index,
		hash:  hash,
	}

	// set as the initial ring or append to the end
	set.lock.Lock()
	defer set.lock.Unlock()

	if set.blocks == nil {
		set.blocks = item
	} else {
		set.blocks.Move(-1).Link(item)
	}
	// Display a log for the user to notify of a new mined block unconfirmed
	log.Info("🔨 mined potential block", "number", index, "hash", hash.Hex())
}

// Shift drops all unconfirmed blocks from the set which exceed the unconfirmed sets depth
// allowance, checking them against the canonical chain for inclusion or staleness
// report.
func (set *unconfirmedBlocks) Shift(height uint64) {
	set.lock.Lock()
	defer set.lock.Unlock()

	for set.blocks != nil {
		// Retrieve the next unconfirmed block and abort if too fresh
		blk := set.blocks.Value.(*unconfirmedBlock)
		// the ring buffer only contains #depth blocks
		if blk.index+uint64(set.depth) > height {
			break
		}
		// block seems to exceed depth allowance, check for canonical status
		header := set.chain.GetHeaderByNumber(blk.index)
		switch {
		case header == nil:
			log.Warn("Failed to retrieve header of mined block", "number", blk.index, "hash", blk.hash.Hex())
		case header.Hash() == blk.hash:
			log.Info("🔗 block reached canonical chain", "number", blk.index, "hash", blk.hash.Hex())
		default:
			log.Info("⑂ block  became a side fork", "number", blk.index, "hash", blk.hash.Hex())
		}
		// drop the block out of the ring
		// edge case when 1 == blocks.Len()
		if set.blocks.Value == set.blocks.Next().Value {
			set.blocks = nil
		} else {
			set.blocks = set.blocks.Move(-1)
			set.blocks.Unlink(1)
			set.blocks = set.blocks.Move(1)
		}
	}
}
