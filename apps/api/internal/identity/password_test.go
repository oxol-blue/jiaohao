package identity

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("diner123")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("diner123", hash) {
		t.Fatal("expected matching password to verify")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("expected mismatch to fail")
	}
}
