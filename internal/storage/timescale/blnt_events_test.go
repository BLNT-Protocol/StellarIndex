package timescale

import (
	"strings"
	"testing"
	"time"
)

func TestValidateBLNTEventRoleKinds(t *testing.T) {
	t.Parallel()
	base := BLNTEvent{
		ContractID:      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		Ledger:          1,
		LedgerCloseTime: time.Unix(1, 0),
		TxHash:          strings.Repeat("a", 64),
	}
	for role, kinds := range blntRoleEventKinds {
		for kind := range kinds {
			e := base
			e.ContractRole, e.EventKind = role, kind
			if err := validateBLNTEvent(e); err != nil {
				t.Errorf("valid %s/%s refused: %v", role, kind, err)
			}
		}
	}

	for name, mutate := range map[string]func(*BLNTEvent){
		"empty contract": func(e *BLNTEvent) { e.ContractID = "" },
		"empty tx":       func(e *BLNTEvent) { e.TxHash = "" },
		"zero close time": func(e *BLNTEvent) {
			e.LedgerCloseTime = time.Time{}
		},
		"unknown role": func(e *BLNTEvent) { e.ContractRole = "blend" },
		"wrong role":   func(e *BLNTEvent) { e.EventKind = "deploy" },
		"unknown tier": func(e *BLNTEvent) { e.Tier = "junior" },
		"auction type": func(e *BLNTEvent) {
			v := uint32(4)
			e.AuctionType = &v
		},
	} {
		e := base
		e.ContractRole, e.EventKind = "pool", "supply"
		mutate(&e)
		if err := validateBLNTEvent(e); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
