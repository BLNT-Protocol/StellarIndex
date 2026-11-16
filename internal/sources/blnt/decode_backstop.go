package blnt

import (
	"fmt"

	"github.com/Stellar-Index/StellarIndex/internal/events"
	"github.com/Stellar-Index/StellarIndex/internal/scval"
)

func decodeBackstopEvent(ev *events.Event, out *Event) error { //nolint:gocyclo,cyclop // audited ABI dispatch table.
	switch out.EventType {
	case "migration_activated":
		if err := requireTopics(ev, 1); err != nil {
			return err
		}
		values, err := bodyTuple(ev, 2)
		if err != nil {
			return err
		}
		out.Attributes["activated_at"], err = scval.AsU64(values[0])
		if err != nil {
			return fmt.Errorf("activated_at: %w", err)
		}
		out.Attributes["backfill_end"], err = scval.AsU64(values[1])
		return err
	case "backfill_funded", "distribute":
		if err := requireTopics(ev, 1); err != nil {
			return err
		}
		var err error
		out.Amount, err = bodyAmount(ev)
		return err
	case "deposit", "queue_withdrawal", "dequeue_withdrawal", "withdraw":
		return decodeBackstopBalanceEvent(ev, out)
	case "gulp_emissions":
		if err := requireTopics(ev, 2); err != nil {
			return err
		}
		var err error
		if out.Pool, err = topicAddress(ev, 1); err != nil {
			return fmt.Errorf("pool: %w", err)
		}
		amounts, err := tupleAmounts(ev, 2)
		if err != nil {
			return err
		}
		out.Amount, out.Amount2 = amounts[0], amounts[1]
		return nil
	case "rw_zone_add":
		return decodeBackstopRewardZoneAdd(ev, out)
	case "rw_zone_remove":
		if err := requireTopics(ev, 1); err != nil {
			return err
		}
		var err error
		out.Pool, err = bodyAddress(ev)
		return err
	case "claim":
		return decodeBackstopClaim(ev, out)
	case "draw":
		return decodeBackstopDraw(ev, out)
	case "donate":
		return decodeBackstopDonate(ev, out)
	default:
		return ErrNotBLNTEvent
	}
}

func decodeBackstopBalanceEvent(ev *events.Event, out *Event) error {
	if err := decodeBackstopActorTopics(ev, out, 4, 2, 3); err != nil {
		return err
	}
	var err error
	switch out.EventType {
	case "deposit", "withdraw":
		amounts, decodeErr := tupleAmounts(ev, 2)
		if decodeErr != nil {
			return decodeErr
		}
		out.Amount, out.Amount2 = amounts[0], amounts[1]
	case "dequeue_withdrawal":
		out.Amount, err = bodyAmount(ev)
	case "queue_withdrawal":
		var values []scval.ScVal
		values, err = bodyTuple(ev, 2)
		if err == nil {
			out.Amount, err = amount(values[0])
		}
		if err == nil {
			out.Attributes["expiration"], err = scval.AsU64(values[1])
		}
	}
	return err
}

func decodeBackstopActorTopics(ev *events.Event, out *Event, topicCount, poolIndex, userIndex int) error {
	if err := requireTopics(ev, topicCount); err != nil {
		return err
	}
	var err error
	if out.Tier, err = decodeTier(ev, 1); err != nil {
		return fmt.Errorf("tier: %w", err)
	}
	if out.Pool, err = topicAddress(ev, poolIndex); err != nil {
		return fmt.Errorf("pool: %w", err)
	}
	if out.UserAddress, err = topicAddress(ev, userIndex); err != nil {
		return fmt.Errorf("user: %w", err)
	}
	return nil
}

func decodeBackstopRewardZoneAdd(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 1); err != nil {
		return err
	}
	values, err := bodyTuple(ev, 2)
	if err != nil {
		return err
	}
	if out.Pool, err = scval.AsAddressStrkey(values[0]); err != nil {
		return fmt.Errorf("to_add: %w", err)
	}
	if out.Target, err = scval.AsAddressOrVoid(values[1]); err != nil {
		return fmt.Errorf("to_remove: %w", err)
	}
	return nil
}

func decodeBackstopClaim(ev *events.Event, out *Event) error {
	if err := decodeBackstopActorTopics(ev, out, 4, 3, 2); err != nil {
		return err
	}
	amounts, err := tupleAmounts(ev, 3)
	if err != nil {
		return err
	}
	out.Amount, out.Amount2, out.Amount3 = amounts[0], amounts[1], amounts[2]
	return nil
}

func decodeBackstopDraw(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 3); err != nil {
		return err
	}
	var err error
	if out.Tier, err = decodeTier(ev, 1); err != nil {
		return fmt.Errorf("tier: %w", err)
	}
	if out.Pool, err = topicAddress(ev, 2); err != nil {
		return fmt.Errorf("pool: %w", err)
	}
	values, err := bodyTuple(ev, 2)
	if err != nil {
		return err
	}
	if out.Target, err = scval.AsAddressStrkey(values[0]); err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	if out.Amount, err = amount(values[1]); err != nil {
		return fmt.Errorf("amount: %w", err)
	}
	return nil
}

func decodeBackstopDonate(ev *events.Event, out *Event) error {
	if err := decodeBackstopActorTopics(ev, out, 4, 2, 3); err != nil {
		return err
	}
	var err error
	out.Amount, err = bodyAmount(ev)
	return err
}
