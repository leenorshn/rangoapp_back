package authn

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestDevelopmentOTPSeed(t *testing.T) {
	svc := &Service{cfg: Config{SeededOTP: DevelopmentOTPSeed()}}
	code, err := svc.issueCode("+243978154329")
	if err != nil {
		t.Fatal(err)
	}
	if code != "123123" {
		t.Fatalf("seed code %s", code)
	}
	other, err := svc.issueCode("+243970000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 6 {
		t.Fatalf("random code %s", other)
	}
	plain := &Service{cfg: Config{}}
	if _, ok := plain.cfg.seededCode("+243978154329"); ok {
		t.Fatal("empty seed accepted the development number")
	}
}

func TestPasswordRejectsOverlongInput(t *testing.T) {
	over := strings.Repeat("a", bcryptMaxBytes+1)
	if _, err := hashPassword(over, bcrypt.MinCost); err == nil {
		t.Fatal("password longer than 72 bytes was hashed")
	}
	short := "court"
	if _, err := hashPassword(short, bcrypt.MinCost); err == nil {
		t.Fatal("short password was hashed")
	}
	hashed, err := hashPassword("motdepasse", bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPassword(hashed, "motdepasse"); err != nil {
		t.Fatal(err)
	}
	if err := checkPassword(hashed, strings.Repeat("a", bcryptMaxBytes+1)); err == nil {
		t.Fatal("overlong password was compared by truncation")
	}
}
