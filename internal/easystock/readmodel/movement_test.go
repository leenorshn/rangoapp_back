package readmodel

import "testing"

func TestCanonicalMovementType(t *testing.T) {
	cases := map[string]string{
		"receive":              "receipt",
		"sale":                 "sale",
		"refund":               "sale_return",
		"adjustment":           "inventory_adjustment",
		"inventory_adjustment": "inventory_adjustment",
		"opening":              "opening",
	}
	for stored, want := range cases {
		if got := CanonicalMovementType(stored); got != want {
			t.Fatalf("%s -> %s, want %s", stored, got, want)
		}
	}
}

func TestMovementTypeFilter(t *testing.T) {
	got, err := MovementTypeFilter("receipt")
	if err != nil || len(got) != 3 || got[0] != "receive" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := MovementTypeFilter("transfer"); err == nil {
		t.Fatal("expected unknown type")
	}
	if got, err := MovementTypeFilter("loss"); err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
}
