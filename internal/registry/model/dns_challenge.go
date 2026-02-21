package model

import (
	"time"

	"github.com/google/uuid"
)

// DNSChallenge represents a pending or completed DNS-01 domain ownership challenge.
type DNSChallenge struct {
	ID        uuid.UUID `json:"id"`
	Domain    string    `json:"domain"`
	Token     string    `json:"token"`
	TXTRecord string    `json:"txt_record"`
	TXTHost   string    `json:"txt_host"` // computed; not stored in DB
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
