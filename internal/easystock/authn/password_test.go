package authn

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

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
