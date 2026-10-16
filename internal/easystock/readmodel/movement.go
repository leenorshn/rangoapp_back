package readmodel

import (
	"strings"

	"rangoapp/internal/easystock/domain"
)

// CanonicalMovementType is the name Android reads.
// Stored values stay compatible with receipts, sales, refunds and older adjustments.
func CanonicalMovementType(stored string) string {
	switch stored {
	case "receive", "receipt", "purchase":
		return "receipt"
	case "refund", "sale_return":
		return "sale_return"
	case "adjustment", "inventory_adjustment":
		return "inventory_adjustment"
	default:
		return stored
	}
}

// MovementTypeFilter accepts the stored value or the Android name.
// opening, loss, purchase_return, transfer_out and transfer_in are written by stock commands.
// origin_provenance is recorded, recovered or legacy_unknown. It is not a unit.
func MovementTypeFilter(requested string) ([]string, error) {
	switch strings.TrimSpace(requested) {
	case "":
		return nil, nil
	case "opening":
		return []string{"opening"}, nil
	case "purchase", "receipt", "receive":
		return []string{"receive", "receipt", "purchase"}, nil
	case "sale":
		return []string{"sale"}, nil
	case "sale_return", "refund":
		return []string{"refund", "sale_return"}, nil
	case "purchase_return":
		return []string{"purchase_return"}, nil
	case "inventory_adjustment", "adjustment":
		return []string{"adjustment", "inventory_adjustment"}, nil
	case "loss":
		return []string{"loss"}, nil
	case "transfer_in":
		return []string{"transfer_in"}, nil
	case "transfer_out":
		return []string{"transfer_out"}, nil
	default:
		return nil, domain.Invalid("movement_type", "Type de mouvement inconnu.")
	}
}
