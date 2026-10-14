package phone

import "testing"

func TestNormalizeDRC(t *testing.T) {
	got, err := Normalize("0970000001", "CD")
	if err != nil {
		t.Fatal(err)
	}
	if got != "+243970000001" {
		t.Fatalf("got %s", got)
	}
	again, err := Normalize("+243 97 000 00 01", "CD")
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Fatalf("international %s local %s", again, got)
	}
	if _, err := Normalize("123", "CD"); err == nil {
		t.Fatal("short number was accepted")
	}
	if _, err := Normalize("0970000001", ""); err == nil {
		t.Fatal("missing region was accepted")
	}
	seed, err := Normalize("+243978154329", "CD")
	if err != nil {
		t.Fatal(err)
	}
	if seed != "+243978154329" {
		t.Fatalf("seed phone %s", seed)
	}
	local, err := Normalize("0978154329", "CD")
	if err != nil {
		t.Fatal(err)
	}
	if local != seed {
		t.Fatalf("local seed %s", local)
	}
}
