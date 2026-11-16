package blnt

import (
	"fmt"

	"github.com/Stellar-Index/StellarIndex/internal/events"
	"github.com/Stellar-Index/StellarIndex/internal/scval"
)

type assetAmountAttribute struct {
	Asset  string `json:"asset"`
	Amount string `json:"amount"`
}

func decodePoolEvent(ev *events.Event, out *Event) error { //nolint:gocyclo,cyclop // audited ABI dispatch table.
	switch out.EventType {
	case "supply", "withdraw", "supply_collateral", "withdraw_collateral", "borrow", "repay", "flash_loan":
		return decodePoolPosition(ev, out)
	case "clawback":
		return decodePoolClawback(ev, out)
	case "gulp", "bad_debt", "defaulted_debt", "protocol_credit_loss":
		return decodePoolSingleAmount(ev, out)
	case "reconcile_loss":
		return decodePoolLoss(ev, out)
	case "claim":
		return decodePoolClaim(ev, out)
	case "reserve_emission_update":
		return decodePoolReserveEmission(ev, out)
	case "gulp_emissions":
		return decodePoolGulpEmissions(ev, out)
	case "set_admin", "update_pool", "queue_set_reserve", "cancel_set_reserve", "set_reserve", "set_status":
		return decodePoolAdmin(ev, out)
	case "new_auction", "fill_auction", "delete_auction":
		return decodePoolAuction(ev, out)
	default:
		return ErrNotBLNTEvent
	}
}

func decodePoolPosition(ev *events.Event, out *Event) error {
	wantTopics := 3
	if out.EventType == "flash_loan" {
		wantTopics = 4
	}
	if err := requireTopics(ev, wantTopics); err != nil {
		return err
	}
	var err error
	if out.Asset, err = topicAddress(ev, 1); err != nil {
		return fmt.Errorf("asset: %w", err)
	}
	if out.UserAddress, err = topicAddress(ev, 2); err != nil {
		return fmt.Errorf("user: %w", err)
	}
	if wantTopics == 4 {
		if out.Counterparty, err = topicAddress(ev, 3); err != nil {
			return fmt.Errorf("counterparty: %w", err)
		}
	}
	amounts, err := tupleAmounts(ev, 2)
	if err != nil {
		return err
	}
	out.Amount, out.Amount2 = amounts[0], amounts[1]
	return nil
}

func decodePoolClawback(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 3); err != nil {
		return err
	}
	var err error
	if out.Asset, err = topicAddress(ev, 1); err != nil {
		return fmt.Errorf("asset: %w", err)
	}
	if out.UserAddress, err = topicAddress(ev, 2); err != nil {
		return fmt.Errorf("user: %w", err)
	}
	amounts, err := tupleAmounts(ev, 3)
	if err != nil {
		return err
	}
	out.Amount, out.Amount2, out.Amount3 = amounts[0], amounts[1], amounts[2]
	return nil
}

func decodePoolSingleAmount(ev *events.Event, out *Event) error {
	wantTopics := 2
	if out.EventType == "bad_debt" {
		wantTopics = 3
	}
	if err := requireTopics(ev, wantTopics); err != nil {
		return err
	}
	var err error
	if out.EventType == "bad_debt" {
		if out.UserAddress, err = topicAddress(ev, 1); err != nil {
			return fmt.Errorf("user: %w", err)
		}
		if out.Asset, err = topicAddress(ev, 2); err != nil {
			return fmt.Errorf("asset: %w", err)
		}
	} else if out.Asset, err = topicAddress(ev, 1); err != nil {
		return fmt.Errorf("asset: %w", err)
	}
	if out.Amount, err = bodyAmount(ev); err != nil {
		return err
	}
	return nil
}

func decodePoolLoss(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 2); err != nil {
		return err
	}
	var err error
	if out.Asset, err = topicAddress(ev, 1); err != nil {
		return fmt.Errorf("asset: %w", err)
	}
	amounts, err := tupleAmounts(ev, 4)
	if err != nil {
		return err
	}
	out.Amount, out.Amount2, out.Amount3, out.Amount4 = amounts[0], amounts[1], amounts[2], amounts[3]
	return nil
}

func decodePoolClaim(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 2); err != nil {
		return err
	}
	var err error
	if out.UserAddress, err = topicAddress(ev, 1); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	values, err := bodyTuple(ev, 2)
	if err != nil {
		return err
	}
	ids, err := scval.AsVec(values[0])
	if err != nil {
		return fmt.Errorf("reserve_token_ids: %w", err)
	}
	decodedIDs := make([]uint32, len(ids))
	for i := range ids {
		if decodedIDs[i], err = scval.AsU32(ids[i]); err != nil {
			return fmt.Errorf("reserve_token_ids[%d]: %w", i, err)
		}
	}
	if out.Amount, err = amount(values[1]); err != nil {
		return fmt.Errorf("amount_claimed: %w", err)
	}
	out.Attributes["reserve_token_ids"] = decodedIDs
	return nil
}

func decodePoolReserveEmission(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 1); err != nil {
		return err
	}
	values, err := bodyTuple(ev, 3)
	if err != nil {
		return err
	}
	resTokenID, err := scval.AsU32(values[0])
	if err != nil {
		return fmt.Errorf("res_token_id: %w", err)
	}
	eps, err := scval.AsU64(values[1])
	if err != nil {
		return fmt.Errorf("eps: %w", err)
	}
	expiration, err := scval.AsU64(values[2])
	if err != nil {
		return fmt.Errorf("expiration: %w", err)
	}
	out.Attributes["res_token_id"] = resTokenID
	out.Attributes["emissions_per_second"] = eps
	out.Attributes["expiration"] = expiration
	return nil
}

func decodePoolGulpEmissions(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 1); err != nil {
		return err
	}
	var err error
	out.Amount, err = bodyAmount(ev)
	return err
}

func decodePoolAdmin(ev *events.Event, out *Event) error { //nolint:gocyclo,cyclop // six fixed ABI variants.
	var err error
	switch out.EventType {
	case "set_admin":
		if err = requireTopics(ev, 2); err == nil {
			out.Admin, err = topicAddress(ev, 1)
		}
		if err == nil {
			out.Target, err = bodyAddress(ev)
		}
	case "update_pool":
		if err = requireTopics(ev, 2); err == nil {
			out.Admin, err = topicAddress(ev, 1)
		}
		var values []scval.ScVal
		if err == nil {
			values, err = bodyTuple(ev, 3)
		}
		if err == nil {
			var rate, positions uint32
			rate, err = scval.AsU32(values[0])
			if err == nil {
				positions, err = scval.AsU32(values[1])
			}
			if err == nil {
				out.Amount, err = amount(values[2])
			}
			out.Attributes["backstop_take_rate"] = rate
			out.Attributes["max_positions"] = positions
		}
	case "queue_set_reserve":
		if err = requireTopics(ev, 2); err == nil {
			out.Admin, err = topicAddress(ev, 1)
		}
		var values []scval.ScVal
		if err == nil {
			values, err = bodyTuple(ev, 2)
		}
		if err == nil {
			out.Asset, err = scval.AsAddressStrkey(values[0])
		}
		if err == nil {
			out.Attributes["reserve_config"], err = decodeReserveConfig(values[1])
		}
	case "cancel_set_reserve":
		if err = requireTopics(ev, 2); err == nil {
			out.Admin, err = topicAddress(ev, 1)
		}
		if err == nil {
			out.Asset, err = bodyAddress(ev)
		}
	case "set_reserve":
		if err = requireTopics(ev, 1); err == nil {
			var values []scval.ScVal
			values, err = bodyTuple(ev, 2)
			if err == nil {
				out.Asset, err = scval.AsAddressStrkey(values[0])
			}
			if err == nil {
				out.Attributes["reserve_index"], err = scval.AsU32(values[1])
			}
		}
	case "set_status":
		if len(ev.Topic) != 1 && len(ev.Topic) != 2 {
			return fmt.Errorf("expected 1 or 2 topics, got %d", len(ev.Topic))
		}
		if len(ev.Topic) == 2 {
			out.Admin, err = topicAddress(ev, 1)
		}
		if err == nil {
			out.Attributes["status"], err = bodyU32(ev)
		}
	}
	return err
}

var reserveConfigFields = []struct {
	name     string
	typeName string
}{
	{"index", "u32"}, {"decimals", "u32"}, {"c_factor", "u32"},
	{"l_factor", "u32"}, {"util", "u32"}, {"max_util", "u32"},
	{"r_base", "u32"}, {"r_one", "u32"}, {"r_two", "u32"},
	{"r_three", "u32"}, {"reactivity", "u32"},
	{"supply_cap", "i128"}, {"enabled", "bool"},
}

func decodeReserveConfig(sv scval.ScVal) (map[string]any, error) {
	entries, err := scval.AsMap(sv)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(reserveConfigFields))
	for _, field := range reserveConfigFields {
		value, err := scval.MustMapField(entries, field.name)
		if err != nil {
			return nil, err
		}
		switch field.typeName {
		case "u32":
			out[field.name], err = scval.AsU32(value)
		case "i128":
			amt, aerr := amount(value)
			err = aerr
			if amt != nil {
				out[field.name] = amt.String()
			}
		case "bool":
			out[field.name], err = scval.AsBool(value)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.name, err)
		}
	}
	return out, nil
}

func decodePoolAuction(ev *events.Event, out *Event) error {
	if err := requireTopics(ev, 3); err != nil {
		return err
	}
	auctionType, err := topicU32(ev, 1)
	if err != nil {
		return fmt.Errorf("auction_type: %w", err)
	}
	if auctionType > 3 {
		return fmt.Errorf("unknown auction_type %d", auctionType)
	}
	out.AuctionType = &auctionType
	if out.UserAddress, err = topicAddress(ev, 2); err != nil {
		return fmt.Errorf("user: %w", err)
	}

	switch out.EventType {
	case "new_auction":
		values, err := bodyTuple(ev, 2)
		if err != nil {
			return err
		}
		percent, err := scval.AsU32(values[0])
		if err != nil {
			return fmt.Errorf("percent: %w", err)
		}
		out.Attributes["percent"] = percent
		return decodeAuctionData(values[1], out)
	case "fill_auction":
		values, err := bodyTuple(ev, 3)
		if err != nil {
			return err
		}
		if out.Counterparty, err = scval.AsAddressStrkey(values[0]); err != nil {
			return fmt.Errorf("filler: %w", err)
		}
		if out.Amount, err = amount(values[1]); err != nil {
			return fmt.Errorf("fill_percent: %w", err)
		}
		return decodeAuctionData(values[2], out)
	case "delete_auction":
		body, err := parseBody(ev)
		if err != nil {
			return err
		}
		if !scval.IsVoid(body) {
			return fmt.Errorf("delete body is not Void")
		}
		return nil
	}
	return ErrNotBLNTEvent
}

func decodeAuctionData(sv scval.ScVal, out *Event) error {
	entries, err := scval.AsMap(sv)
	if err != nil {
		return fmt.Errorf("auction_data: %w", err)
	}
	for _, field := range []string{"bid", "lot"} {
		value, err := scval.MustMapField(entries, field)
		if err != nil {
			return err
		}
		decoded, err := decodeAssetAmountMap(value)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		out.Attributes[field] = decoded
	}
	block, err := scval.MustMapField(entries, "block")
	if err != nil {
		return err
	}
	out.Attributes["block"], err = scval.AsU32(block)
	return err
}

func decodeAssetAmountMap(sv scval.ScVal) ([]assetAmountAttribute, error) {
	entries, err := scval.AsMap(sv)
	if err != nil {
		return nil, err
	}
	out := make([]assetAmountAttribute, len(entries))
	for i := range entries {
		asset, err := scval.AsAddressStrkey(entries[i].Key)
		if err != nil {
			return nil, fmt.Errorf("entry[%d] asset: %w", i, err)
		}
		amt, err := amount(entries[i].Val)
		if err != nil {
			return nil, fmt.Errorf("entry[%d] amount: %w", i, err)
		}
		out[i] = assetAmountAttribute{Asset: asset, Amount: amt.String()}
	}
	return out, nil
}
