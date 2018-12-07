package backend

import (
	"time"

	"bitbucket.org/cpchain/chain/commons/log"
	"bitbucket.org/cpchain/chain/types"
	"github.com/ethereum/go-ethereum/common"
)

func waitForEnoughValidator(h *Handler, term uint64) (validators map[common.Address]*RemoteValidator) {
	ready := false
	for !ready {
		time.Sleep(1 * time.Second)

		validators = h.dialer.ValidatorsOfTerm(term)

		log.Debug("validators in dpor handler when broadcasting...")
		for addr := range validators {
			log.Debug("validator", "addr", addr.Hex())
		}

		if len(validators) >= int(h.config.TermLen-h.fsm.f) {
			return
		}

	}
	return
}

// BroadcastPreprepareBlock broadcasts generated block to validators
func (h *Handler) BroadcastPreprepareBlock(block *types.Block) {

	log.Debug("proposed new pending block, broadcasting")

	term := h.dpor.TermOf(block.NumberU64())
	validators := waitForEnoughValidator(h, term)

	for _, peer := range validators {
		peer.AsyncSendPreprepareBlock(block)
	}
}

// BroadcastPreprepareImpeachBlock broadcasts generated impeach block to validators
func (h *Handler) BroadcastPreprepareImpeachBlock(block *types.Block) {

	log.Debug("proposed new pending impeach block, broadcasting")

	term := h.dpor.TermOf(block.NumberU64())
	validators := waitForEnoughValidator(h, term)

	for _, peer := range validators {
		peer.AsyncSendPreprepareImpeachBlock(block)
	}
}

// BroadcastPrepareHeader broadcasts signed prepare header to remote validators
func (h *Handler) BroadcastPrepareHeader(header *types.Header) {
	h.lock.Lock()
	defer h.lock.Unlock()

	log.Debug("composed prepare header msg, broadcasting", "number", header.Number.Uint64())

	term := h.dpor.TermOf(header.Number.Uint64())
	validators := h.dialer.ValidatorsOfTerm(term)

	for _, peer := range validators {
		peer.AsyncSendPrepareHeader(header)
	}
}

// BroadcastPrepareImpeachHeader broadcasts signed impeach prepare header to remote validators
func (h *Handler) BroadcastPrepareImpeachHeader(header *types.Header) {
	h.lock.Lock()
	defer h.lock.Unlock()

	log.Debug("composed prepare impeach header msg, broadcasting", "number", header.Number.Uint64())

	term := h.dpor.TermOf(header.Number.Uint64())
	validators := h.dialer.ValidatorsOfTerm(term)

	for _, peer := range validators {
		peer.AsyncSendPrepareImpeachHeader(header)
	}
}

// BroadcastCommitHeader broadcasts signed commit header to remote validators
func (h *Handler) BroadcastCommitHeader(header *types.Header) {
	h.lock.Lock()
	defer h.lock.Unlock()

	log.Debug("composed commit header msg, broadcasting", "number", header.Number.Uint64())

	term := h.dpor.TermOf(header.Number.Uint64())
	validators := h.dialer.ValidatorsOfTerm(term)

	for _, peer := range validators {
		peer.AsyncSendCommitHeader(header)
	}
}

// BroadcastCommitImpeachHeader broadcasts signed impeach commit header to remote validators
func (h *Handler) BroadcastCommitImpeachHeader(header *types.Header) {
	h.lock.Lock()
	defer h.lock.Unlock()

	log.Debug("composed commit impeach header msg, broadcasting", "number", header.Number.Uint64())

	term := h.dpor.TermOf(header.Number.Uint64())
	validators := h.dialer.ValidatorsOfTerm(term)

	for _, peer := range validators {
		peer.AsyncSendCommitImpeachHeader(header)
	}
}

// PendingBlockBroadcastLoop loops to broadcast blocks
func (h *Handler) PendingBlockBroadcastLoop() {
	futureTimer := time.NewTicker(time.Duration(h.dpor.ImpeachTimeout()))
	defer futureTimer.Stop()

	for {
		select {
		case pendingBlock := <-h.pendingBlockCh:

			// broadcast mined pending block to remote signers
			go h.BroadcastPreprepareBlock(pendingBlock)

		case <-futureTimer.C:

			// check if still not received new block, if true, continue
			if h.ReadyToImpeach() && h.mode == PBFTMode {
				// get empty block

				log.Debug("composing preprepare impeach block msg")

				impeachHeader, act, dtype, msg, err := h.fsm.Fsm(nil, 0, ImpeachPreprepareMsgCode)
				_, _, _, _, _ = impeachHeader, act, dtype, msg, err

				if impeachHeader != nil && act == BroadcastMsgAction && dtype == HeaderType && msg == PrepareMsgCode && err == nil {
					header := impeachHeader.(*types.Header)
					go h.BroadcastPrepareImpeachHeader(header)
				}

			}

		case <-h.quitSync:
			return
		}
	}
}
