// Copyright (c) 2026 Stellar Index contributors.
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"log/slog"

	"github.com/Stellar-Index/StellarIndex/internal/obs"
	"github.com/Stellar-Index/StellarIndex/internal/sources/blnt"
	"github.com/Stellar-Index/StellarIndex/internal/storage/timescale"
)

// persistBLNTEvent lands one identity-gated, typed BLNT V3 event. Raw XDR stays
// in the lake and soroban_events; this is never a Blend row or spot-price observation.
func persistBLNTEvent(ctx context.Context, logger *slog.Logger, store *timescale.Store, e blnt.Event) error {
	if err := store.InsertBLNTEvent(ctx, timescale.BLNTEvent{
		ContractID:      e.ContractID,
		Ledger:          e.Ledger,
		LedgerCloseTime: e.ObservedAt,
		TxHash:          e.TxHash,
		OpIndex:         e.OpIndex,
		EventIndex:      e.EventIndex,
		ContractRole:    string(e.Role),
		EventKind:       e.EventType,
		Pool:            e.Pool,
		UserAddress:     e.UserAddress,
		Asset:           e.Asset,
		Counterparty:    e.Counterparty,
		Target:          e.Target,
		Admin:           e.Admin,
		Tier:            string(e.Tier),
		AuctionType:     e.AuctionType,
		Amount:          e.Amount,
		Amount2:         e.Amount2,
		Amount3:         e.Amount3,
		Amount4:         e.Amount4,
		Attributes:      e.Attributes,
	}); err != nil {
		obs.SourceInsertErrorsTotal.WithLabelValues(blnt.SourceName, "blnt_events").Inc()
		logger.Error("insert BLNT V3 event failed",
			"contract_id", e.ContractID, "contract_role", e.Role, "event_type", e.EventType,
			"ledger", e.Ledger, "tx_hash", e.TxHash, "err", err)
		return err
	}
	bumpEntryCount(ctx, logger, store, blnt.SourceName)
	logger.Debug("BLNT V3 event ingested",
		"source", blnt.SourceName, "contract_role", e.Role, "event_type", e.EventType,
		"contract_id", e.ContractID, "ledger", e.Ledger)
	return nil
}
