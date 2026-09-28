package service

import "testing"

func TestOrganizationInvitationTokenIsOpaqueAndHashed(t *testing.T) {
	token, hash, err := newOrganizationInvitationToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if token == "" || hash == "" {
		t.Fatal("token and hash must be populated")
	}
	if token == hash {
		t.Fatal("stored hash must not expose the invitation token")
	}
	if got := organizationInvitationHash(token); got != hash {
		t.Fatalf("hash mismatch: got %q want %q", got, hash)
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
}

func TestOrganizationInvitationTokensAreUnique(t *testing.T) {
	first, _, err := newOrganizationInvitationToken()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, _, err := newOrganizationInvitationToken()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if first == second {
		t.Fatal("generated invitation tokens must be unique")
	}
}
