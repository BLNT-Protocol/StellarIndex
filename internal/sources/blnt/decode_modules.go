package blnt

import (
	"fmt"

	"github.com/Stellar-Index/StellarIndex/internal/canonical"
	"github.com/Stellar-Index/StellarIndex/internal/events"
	"github.com/Stellar-Index/StellarIndex/internal/scval"
)

type recipientAmountAttribute struct {
	Recipient string `json:"recipient"`
	Amount    string `json:"amount"`
}

func decodeEmitterEvent(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 1); err != nil {
		return err
	}
	switch out.EventType {
	case "distribute":
		values, err := bodyTuple(ev, 2)
		if err != nil {
			return err
		}
		if out.Target, err = scval.AsAddressStrkey(values[0]); err != nil {
			return fmt.Errorf("backstop: %w", err)
		}
		if out.Amount, err = amount(values[1]); err != nil {
			return fmt.Errorf("amount: %w", err)
		}
		return nil
	case "q_swap", "del_swap", "swap":
		return decodeEmitterSwap(ev, out)
	case "drop":
		return decodeEmitterDrop(ev, out)
	default:
		return ErrNotBLNTEvent
	}
}

func decodeEmitterSwap(ev *events.Event, out *Event) error {
	body, err := parseBody(ev)
	if err != nil {
		return err
	}
	entries, err := scval.AsMap(body)
	if err != nil {
		return err
	}
	newBackstop, err := scval.MustMapField(entries, "new_backstop")
	if err != nil {
		return err
	}
	if out.Target, err = scval.AsAddressStrkey(newBackstop); err != nil {
		return fmt.Errorf("new_backstop: %w", err)
	}
	newToken, err := scval.MustMapField(entries, "new_backstop_token")
	if err != nil {
		return err
	}
	if out.Asset, err = scval.AsAddressStrkey(newToken); err != nil {
		return fmt.Errorf("new_backstop_token: %w", err)
	}
	unlockTime, err := scval.MustMapField(entries, "unlock_time")
	if err != nil {
		return err
	}
	out.Attributes["unlock_time"], err = scval.AsU64(unlockTime)
	return err
}

func decodeEmitterDrop(ev *events.Event, out *Event) error {
	body, err := parseBody(ev)
	if err != nil {
		return err
	}
	values, err := scval.AsVec(body)
	if err != nil {
		return err
	}
	recipients := make([]recipientAmountAttribute, len(values))
	for i := range values {
		pair, err := scval.AsTupleN(values[i], 2)
		if err != nil {
			return fmt.Errorf("recipient[%d]: %w", i, err)
		}
		recipients[i].Recipient, err = scval.AsAddressStrkey(pair[0])
		if err != nil {
			return fmt.Errorf("recipient[%d] address: %w", i, err)
		}
		amt, err := amount(pair[1])
		if err != nil {
			return fmt.Errorf("recipient[%d] amount: %w", i, err)
		}
		recipients[i].Amount = amt.String()
	}
	out.Attributes["recipients"] = recipients
	return nil
}

func decodeBackfillEvent(ev *events.Event, out *Event) error {
	switch out.EventType {
	case "claim", "claim_grant":
		if err := requireTopics(ev, 2); err != nil {
			return err
		}
		var err error
		if out.UserAddress, err = topicAddress(ev, 1); err != nil {
			return fmt.Errorf("user: %w", err)
		}
		out.Amount, err = bodyAmount(ev)
		return err
	case "swap_blnd":
		if err := requireTopics(ev, 2); err != nil {
			return err
		}
		var err error
		if out.UserAddress, err = topicAddress(ev, 1); err != nil {
			return fmt.Errorf("user: %w", err)
		}
		amounts, err := tupleAmounts(ev, 3)
		if err != nil {
			return err
		}
		out.Amount, out.Amount2, out.Amount3 = amounts[0], amounts[1], amounts[2]
		return nil
	case "burn_expired":
		if err := requireTopics(ev, 1); err != nil {
			return err
		}
		amounts, err := tupleAmounts(ev, 2)
		if err != nil {
			return err
		}
		out.Amount, out.Amount2 = amounts[0], amounts[1]
		return nil
	default:
		return ErrNotBLNTEvent
	}
}

func decodeLockupBackfillEvent(ev *events.Event, out *Event) error {
	switch out.EventType {
	case "fund", "claim", "expire":
		if err := requireTopics(ev, 2); err != nil {
			return err
		}
		var err error
		if out.UserAddress, err = topicAddress(ev, 1); err != nil {
			return fmt.Errorf("user: %w", err)
		}
		amounts, err := tupleAmounts(ev, 2)
		if err != nil {
			return err
		}
		out.Amount, out.Amount2 = amounts[0], amounts[1]
		return nil
	case "claim_fund":
		if err := requireTopics(ev, 1); err != nil {
			return err
		}
		return decodeClaimFund(ev, out)
	default:
		return ErrNotBLNTEvent
	}
}

func decodeClaimFund(ev *events.Event, out *Event) error {
	body, err := parseBody(ev)
	if err != nil {
		return err
	}
	entries, err := scval.AsMap(body)
	if err != nil {
		return err
	}
	fields := []string{"grant_claimed", "lockup_funded", "manager_funded", "owner_retained"}
	amounts := make([]*canonical.Amount, len(fields))
	for i := range fields {
		value, err := scval.MustMapField(entries, fields[i])
		if err != nil {
			return err
		}
		amounts[i], err = amount(value)
		if err != nil {
			return fmt.Errorf("%s: %w", fields[i], err)
		}
	}
	out.Amount, out.Amount2, out.Amount3, out.Amount4 = amounts[0], amounts[1], amounts[2], amounts[3]
	return nil
}
