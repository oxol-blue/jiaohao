package identity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAndParseToken(t *testing.T) {
	user := User{
		ID:           uuid.New(),
		StudentID:    "20260001",
		Role:         RoleDiner,
		TokenVersion: 1,
	}
	raw, err := IssueToken("test-secret-123456", time.Hour, user)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken("test-secret-123456", raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Role != RoleDiner || claims.StudentID != "20260001" || claims.TokenVersion != 1 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := ParseToken("other-secret-123456", raw); err == nil {
		t.Fatal("expected parse with wrong secret to fail")
	}
}
