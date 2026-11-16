package timescale

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Stellar-Index/StellarIndex/internal/canonical"
)

// BLNTEvent is one typed row in blnt_events. Raw event XDR remains in the lake
// and soroban_events; this projection stores decoded common columns plus JSON
// attributes for event-specific typed fields.
type BLNTEvent struct {
	ContractID      string
	Ledger          uint32
	LedgerCloseTime time.Time
	TxHash          string
	OpIndex         uint32
	EventIndex      uint32
	ContractRole    string
	EventKind       string

	Pool         string
	UserAddress  string
	Asset        string
	Counterparty string
	Target       string
	Admin        string
	Tier         string
	AuctionType  *uint32

	Amount  *canonical.Amount
	Amount2 *canonical.Amount
	Amount3 *canonical.Amount
	Amount4 *canonical.Amount

	Attributes map[string]any
}

var blntRoleEventKinds = map[string]map[string]struct{}{
	"pool": blntEventKindSet(
		"set_admin", "update_pool", "queue_set_reserve", "cancel_set_reserve", "set_reserve", "set_status",
		"reserve_emission_update", "gulp_emissions", "claim", "bad_debt", "defaulted_debt",
		"supply", "withdraw", "supply_collateral", "withdraw_collateral", "borrow", "repay", "flash_loan",
		"clawback", "gulp", "reconcile_loss", "protocol_credit_loss",
		"new_auction", "fill_auction", "delete_auction",
	),
	"factory": blntEventKindSet("deploy"),
	"backstop": blntEventKindSet(
		"migration_activated", "backfill_funded", "deposit", "queue_withdrawal", "dequeue_withdrawal", "withdraw",
		"distribute", "gulp_emissions", "rw_zone_add", "rw_zone_remove", "claim", "draw", "donate",
	),
	"emitter":         blntEventKindSet("distribute", "q_swap", "del_swap", "swap", "drop"),
	"backfill":        blntEventKindSet("claim", "claim_grant", "swap_blnd", "burn_expired"),
	"lockup_backfill": blntEventKindSet("fund", "claim", "claim_fund", "expire"),
}

func blntEventKindSet(kinds ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		out[kind] = struct{}{}
	}
	return out
}

// InsertBLNTEvent appends one typed BLNT V3 event, idempotent on its ledger
// identity. Equal-or-higher derive generations correct a prior projection; a
// lower generation cannot overwrite it.
func (s *Store) InsertBLNTEvent(ctx context.Context, e BLNTEvent) error {
	if err := validateBLNTEvent(e); err != nil {
		return fmt.Errorf("timescale: InsertBLNTEvent: %w", err)
	}
	attrs := []byte("{}")
	if len(e.Attributes) > 0 {
		var err error
		attrs, err = json.Marshal(e.Attributes)
		if err != nil {
			return fmt.Errorf("timescale: InsertBLNTEvent: marshal attributes: %w", err)
		}
	}

	const q = `
        INSERT INTO blnt_events (
            contract_id, ledger, ledger_close_time, tx_hash, op_index, event_index,
            contract_role, event_kind,
            pool, user_address, asset, counterparty, target, admin, tier, auction_type,
            amount, amount2, amount3, amount4, attributes, derive_generation
        ) VALUES (
            $1, $2, $3, $4, $5, $6,
            $7, $8,
            $9, $10, $11, $12, $13, $14, $15, $16,
            $17, $18, $19, $20, $21, $22
        )
        ON CONFLICT (ledger_close_time, contract_id, ledger, tx_hash,
                     op_index, event_index) DO UPDATE SET
            contract_role     = EXCLUDED.contract_role,
            event_kind        = EXCLUDED.event_kind,
            pool              = EXCLUDED.pool,
            user_address      = EXCLUDED.user_address,
            asset             = EXCLUDED.asset,
            counterparty      = EXCLUDED.counterparty,
            target            = EXCLUDED.target,
            admin             = EXCLUDED.admin,
            tier              = EXCLUDED.tier,
            auction_type      = EXCLUDED.auction_type,
            amount            = EXCLUDED.amount,
            amount2           = EXCLUDED.amount2,
            amount3           = EXCLUDED.amount3,
            amount4           = EXCLUDED.amount4,
            attributes        = EXCLUDED.attributes,
            derive_generation = EXCLUDED.derive_generation
          WHERE blnt_events.derive_generation <= EXCLUDED.derive_generation
    `
	_, err := s.db.ExecContext(ctx, q,
		e.ContractID, int(e.Ledger), e.LedgerCloseTime.UTC(), e.TxHash,
		int(e.OpIndex), int(e.EventIndex), e.ContractRole, e.EventKind,
		nullString(e.Pool), nullString(e.UserAddress), nullString(e.Asset),
		nullString(e.Counterparty), nullString(e.Target), nullString(e.Admin),
		nullString(e.Tier), nullUint32(e.AuctionType),
		nullCanonicalAmount(e.Amount), nullCanonicalAmount(e.Amount2),
		nullCanonicalAmount(e.Amount3), nullCanonicalAmount(e.Amount4),
		attrs, s.deriveGeneration,
	)
	if err != nil {
		return fmt.Errorf("timescale: InsertBLNTEvent %s@%d: %w", e.ContractID, e.Ledger, err)
	}
	return nil
}

func validateBLNTEvent(e BLNTEvent) error {
	if e.ContractID == "" {
		return errors.New("ContractID is empty")
	}
	if e.TxHash == "" {
		return errors.New("TxHash is empty")
	}
	kinds, ok := blntRoleEventKinds[e.ContractRole]
	if !ok {
		return fmt.Errorf("invalid ContractRole %q", e.ContractRole)
	}
	if _, ok := kinds[e.EventKind]; !ok {
		return fmt.Errorf("EventKind %q cannot be emitted by role %q", e.EventKind, e.ContractRole)
	}
	if e.LedgerCloseTime.IsZero() {
		return errors.New("LedgerCloseTime is zero")
	}
	if e.Tier != "" && e.Tier != "first_loss" && e.Tier != "second_loss" && e.Tier != "third_loss" {
		return fmt.Errorf("invalid Tier %q", e.Tier)
	}
	if e.AuctionType != nil && *e.AuctionType > 3 {
		return fmt.Errorf("invalid AuctionType %d", *e.AuctionType)
	}
	return nil
}

func nullUint32(v *uint32) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullCanonicalAmount(v *canonical.Amount) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: v.String(), Valid: true}
}
