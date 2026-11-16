package blnt

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stellar-Index/StellarIndex/internal/events"
	"github.com/Stellar-Index/StellarIndex/internal/scval"
)

const fixtureRoot = "../../../test/fixtures/blnt"

type realFixture struct {
	ContractID     string   `json:"contract_id"`
	Role           Role     `json:"role"`
	WASMHash       string   `json:"wasm_hash"`
	Ledger         uint32   `json:"ledger"`
	TxHash         string   `json:"tx_hash"`
	OpIndex        int      `json:"op_index"`
	EventIndex     int      `json:"event_index"`
	LedgerClosedAt string   `json:"ledger_closed_at"`
	Topics         []string `json:"topics"`
	Value          string   `json:"value"`
	EventName      string   `json:"event_name"`
}

func (f realFixture) event() events.Event {
	return events.Event{
		Type: "contract", ContractID: f.ContractID, Ledger: f.Ledger,
		LedgerClosedAt: f.LedgerClosedAt, TxHash: f.TxHash,
		OperationIndex: f.OpIndex, EventIndex: f.EventIndex,
		Topic: f.Topics, Value: f.Value,
		InSuccessfulContractCall: true,
	}
}

func readRealFixture(t *testing.T, path string) realFixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f realFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return f
}

// These records were exported from the public Galexie archive, not synthesized.
// Every immutable BLNT V3 role and audited WASM lineage must remain represented.
func TestRealFixturesDecodeEveryAuditedRole(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join(fixtureRoot, "*", "*.json"))
	if err != nil || len(files) < 10 {
		t.Fatalf("fixtures = %d (err %v), want at least 10 Galexie records", len(files), err)
	}
	wantHashes := map[string]Role{
		"4fa7d99dee6fe99cad5d07a46ee39e437dc287e53e3ab1f3ccc0aacf7e6cd6c5": RoleEmitter,
		"dcdc0b1860799629c7c88804955dd7373cb6febaf069e2ba863e98ebbaa95c27": RoleBackfill,
		"985d7eda4b610f28103edcb28faa52408169f455ac9a880c4a39f58ecc0c2135": RoleFactory,
		"0469cfb445bedd87478274f265c8776b32cde1e8840fca4684ffa1dc4d5a750f": RoleBackstop,
		"0e4f8f45b94e6b0d017a496ac5620ad0e3f866533cf45acd2b070bedda1e7356": RolePool,
		"7c3d3591b6227827c21a0fe0da5df402bd55394fa5ed38c0bbace0400a823ddb": RoleLockupBackfill,
	}
	seen := make(map[string]bool, len(wantHashes))
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			f := readRealFixture(t, path)
			hash := filepath.Base(filepath.Dir(path))
			wantRole, known := wantHashes[hash]
			if !known || f.WASMHash != hash || f.Role != wantRole {
				t.Fatalf("fixture path/hash/role mismatch: path=%s hash=%s role=%s", path, f.WASMHash, f.Role)
			}
			seen[hash] = true
			if !strings.HasPrefix(filepath.Base(path), f.EventName+"_") || len(f.Topics) == 0 ||
				f.Topics[0] != scval.MustEncodeSymbol(f.EventName) {
				t.Fatalf("event name/topic/path mismatch for %s", path)
			}
			for _, raw := range append([]string{f.Value}, f.Topics...) {
				if _, err := base64.StdEncoding.DecodeString(raw); err != nil {
					t.Fatalf("invalid base64 XDR: %v", err)
				}
			}

			d := NewDecoder()
			ev := f.event()
			if !d.Matches(ev) {
				t.Fatalf("decoder did not claim %s/%s", f.Role, f.EventName)
			}
			out, err := d.Decode(ev)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("Decode emitted %d events, want 1", len(out))
			}
			got := out[0].(Event)
			if got.Role != f.Role || got.EventType != f.EventName || got.ContractID != f.ContractID ||
				got.Ledger != f.Ledger || got.TxHash != f.TxHash || int(got.OpIndex) != f.OpIndex ||
				int(got.EventIndex) != f.EventIndex {
				t.Fatalf("decoded identity differs from Galexie fixture: %+v", got)
			}
			if got.ObservedAt.UTC().Format("2006-01-02T15:04:05Z") != f.LedgerClosedAt {
				t.Fatalf("close time = %s, want %s", got.ObservedAt, f.LedgerClosedAt)
			}
			assertFixtureTypedProjection(t, got)
		})
	}
	for hash, role := range wantHashes {
		if !seen[hash] {
			t.Errorf("no real fixture for %s WASM %s", role, hash)
		}
	}
}

func assertFixtureTypedProjection(t *testing.T, got Event) {
	t.Helper()
	if got.Attributes == nil {
		t.Fatal("attributes map is nil")
	}
	switch got.Role + "/" + Role(got.EventType) {
	case RoleBackstop + "/deposit":
		if got.Pool == "" || got.UserAddress == "" || got.Tier == "" || got.Amount == nil || got.Amount2 == nil {
			t.Fatalf("deposit typed projection is incomplete: %+v", got)
		}
	case RolePool + "/flash_loan":
		if got.Asset == "" || got.UserAddress == "" || got.Counterparty == "" || got.Amount == nil || got.Amount2 == nil {
			t.Fatalf("flash-loan typed projection is incomplete: %+v", got)
		}
	case RolePool + "/repay", RolePool + "/supply_collateral":
		if got.Asset == "" || got.UserAddress == "" || got.Amount == nil || got.Amount2 == nil {
			t.Fatalf("pool-position typed projection is incomplete: %+v", got)
		}
	case RoleEmitter + "/distribute":
		if got.Target == "" || got.Amount == nil {
			t.Fatalf("emitter distribution typed projection is incomplete: %+v", got)
		}
	case RoleLockupBackfill + "/claim_fund":
		if got.Amount == nil || got.Amount2 == nil || got.Amount3 == nil || got.Amount4 == nil {
			t.Fatalf("claim_fund typed projection is incomplete: %+v", got)
		}
	case RoleLockupBackfill + "/fund":
		if got.UserAddress == "" || got.Amount == nil || got.Amount2 == nil {
			t.Fatalf("lockup fund typed projection is incomplete: %+v", got)
		}
	case RoleFactory + "/deploy":
		if got.Target == "" {
			t.Fatalf("factory deploy typed projection is incomplete: %+v", got)
		}
	case RoleBackfill + "/claim":
		if got.UserAddress == "" || got.Amount == nil {
			t.Fatalf("backfill claim typed projection is incomplete: %+v", got)
		}
	case RoleBackfill + "/swap_blnd":
		if got.UserAddress == "" || got.Amount == nil || got.Amount2 == nil || got.Amount3 == nil {
			t.Fatalf("backfill swap typed projection is incomplete: %+v", got)
		}
	default:
		t.Fatalf("fixture lacks typed-projection assertion for %s/%s", got.Role, got.EventType)
	}
}
