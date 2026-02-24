package model

import (
	"time"

	"github.com/google/uuid"
)

// AbuseReportStatus represents the lifecycle state of an abuse report.
type AbuseReportStatus string

const (
	AbuseStatusOpen          AbuseReportStatus = "open"
	AbuseStatusInvestigating AbuseReportStatus = "investigating"
	AbuseStatusResolved      AbuseReportStatus = "resolved"
	AbuseStatusDismissed     AbuseReportStatus = "dismissed"
)

// AbuseReport represents an abuse report filed against an agent.
type AbuseReport struct {
	ID             uuid.UUID         `json:"id"               db:"id"`
	AgentID        uuid.UUID         `json:"agent_id"         db:"agent_id"`
	ReporterUserID uuid.UUID         `json:"reporter_user_id" db:"reporter_user_id"`
	Reason         string            `json:"reason"           db:"reason"`
	Details        string            `json:"details"          db:"details"`
	Status         AbuseReportStatus `json:"status"           db:"status"`
	ResolutionNote string            `json:"resolution_note"  db:"resolution_note"`
	CreatedAt      time.Time         `json:"created_at"       db:"created_at"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty" db:"resolved_at"`
	ResolvedBy     *uuid.UUID        `json:"resolved_by,omitempty" db:"resolved_by"`
}

// CreateAbuseReportRequest is the payload for filing an abuse report.
type CreateAbuseReportRequest struct {
	Reason  string `json:"reason"  binding:"required"`
	Details string `json:"details"`
}

// ResolveAbuseReportRequest is the payload for resolving/dismissing a report.
type ResolveAbuseReportRequest struct {
	Status         AbuseReportStatus `json:"status"          binding:"required"`
	ResolutionNote string            `json:"resolution_note"`
}
