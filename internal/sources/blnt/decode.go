package blnt

import (
	"fmt"

	"github.com/Stellar-Index/StellarIndex/internal/canonical"
	"github.com/Stellar-Index/StellarIndex/internal/events"
	"github.com/Stellar-Index/StellarIndex/internal/scval"
)

// classify resolves topic[0] only within the ABI selected by contract role.
// Contract identity is established by Decoder.roleOf before this is called.
func classify(role Role, ev *events.Event) string {
	if len(ev.Topic) == 0 {
		return ""
	}
	return encodedRoleEvents[role][ev.Topic[0]]
}

func decodeEvent(role Role, kind string, ev *events.Event) (Event, error) {
	if role == "" || kind == "" {
		return Event{}, ErrNotBLNTEvent
	}
	observedAt, err := ev.EventClosedAt()
	if err != nil {
		return Event{}, malformed(role, kind, "close time", err)
	}
	out := Event{
		ContractID: ev.ContractID,
		Ledger:     ev.Ledger,
		TxHash:     ev.TxHash,
		OpIndex:    uint32(ev.OperationIndex), //nolint:gosec // non-negative by Soroban event contract.
		EventIndex: uint32(ev.EventIndex),     //nolint:gosec // non-negative by Soroban event contract.
		ObservedAt: observedAt,
		Role:       role,
		EventType:  kind,
		Attributes: map[string]any{},
	}

	switch role {
	case RolePool:
		err = decodePoolEvent(ev, &out)
	case RoleFactory:
		err = decodeFactoryEvent(ev, &out)
	case RoleBackstop:
		err = decodeBackstopEvent(ev, &out)
	case RoleEmitter:
		err = decodeEmitterEvent(ev, &out)
	case RoleBackfill:
		err = decodeBackfillEvent(ev, &out)
	case RoleLockupBackfill:
		err = decodeLockupBackfillEvent(ev, &out)
	default:
		err = ErrNotBLNTEvent
	}
	if err != nil {
		return Event{}, malformed(role, kind, "payload", err)
	}
	return out, nil
}

func malformed(role Role, kind, field string, err error) error {
	return fmt.Errorf("%w: %s/%s %s: %w", ErrMalformedEvent, role, kind, field, err)
}

func requireTopics(ev *events.Event, n int) error {
	if len(ev.Topic) != n {
		return fmt.Errorf("expected %d topics, got %d", n, len(ev.Topic))
	}
	return nil
}

func parseTopic(ev *events.Event, index int) (scval.ScVal, error) {
	if index < 0 || index >= len(ev.Topic) {
		return scval.ScVal{}, fmt.Errorf("topic[%d] absent (have %d)", index, len(ev.Topic))
	}
	return scval.Parse(ev.Topic[index])
}

func topicAddress(ev *events.Event, index int) (string, error) {
	sv, err := parseTopic(ev, index)
	if err != nil {
		return "", err
	}
	return scval.AsAddressStrkey(sv)
}

func topicU32(ev *events.Event, index int) (uint32, error) {
	sv, err := parseTopic(ev, index)
	if err != nil {
		return 0, err
	}
	return scval.AsU32(sv)
}

func parseBody(ev *events.Event) (scval.ScVal, error) {
	if ev.Value == "" {
		return scval.ScVal{}, fmt.Errorf("empty body")
	}
	return scval.Parse(ev.Value)
}

func bodyTuple(ev *events.Event, n int) ([]scval.ScVal, error) {
	sv, err := parseBody(ev)
	if err != nil {
		return nil, err
	}
	return scval.AsTupleN(sv, n)
}

func tupleAmounts(ev *events.Event, n int) ([]*canonical.Amount, error) {
	values, err := bodyTuple(ev, n)
	if err != nil {
		return nil, err
	}
	out := make([]*canonical.Amount, n)
	for i := range values {
		out[i], err = amount(values[i])
		if err != nil {
			return nil, fmt.Errorf("amount[%d]: %w", i, err)
		}
	}
	return out, nil
}

func amount(sv scval.ScVal) (*canonical.Amount, error) {
	a, err := scval.AsAmountFromI128(sv)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func bodyAmount(ev *events.Event) (*canonical.Amount, error) {
	sv, err := parseBody(ev)
	if err != nil {
		return nil, err
	}
	return amount(sv)
}

func bodyAddress(ev *events.Event) (string, error) {
	sv, err := parseBody(ev)
	if err != nil {
		return "", err
	}
	return scval.AsAddressStrkey(sv)
}

func bodyU32(ev *events.Event) (uint32, error) {
	sv, err := parseBody(ev)
	if err != nil {
		return 0, err
	}
	return scval.AsU32(sv)
}

func decodeTier(ev *events.Event, index int) (Tier, error) {
	sv, err := parseTopic(ev, index)
	if err != nil {
		return "", err
	}
	variant, err := scval.AsTupleN(sv, 1)
	if err != nil {
		return "", fmt.Errorf("BackstopTier: %w", err)
	}
	name, err := scval.AsSymbol(variant[0])
	if err != nil {
		return "", fmt.Errorf("BackstopTier variant: %w", err)
	}
	switch name {
	case "FirstLoss":
		return TierFirstLoss, nil
	case "SecondLoss":
		return TierSecondLoss, nil
	case "ThirdLoss":
		return TierThirdLoss, nil
	default:
		return "", fmt.Errorf("unknown BackstopTier variant %q", name)
	}
}

func decodeFactoryEvent(ev *events.Event, out *Event) error {
	if out.EventType != "deploy" {
		return ErrNotBLNTEvent
	}
	if err := requireTopics(ev, 1); err != nil {
		return err
	}
	target, err := bodyAddress(ev)
	if err != nil {
		return err
	}
	out.Target = target
	return nil
}

func decodeDeployTarget(ev *events.Event) (string, error) {
	if len(ev.Topic) != 1 {
		return "", fmt.Errorf("%w: factory/deploy expected 1 topic, got %d", ErrMalformedEvent, len(ev.Topic))
	}
	target, err := bodyAddress(ev)
	if err != nil {
		return "", fmt.Errorf("%w: factory/deploy target: %w", ErrMalformedEvent, err)
	}
	return target, nil
}
