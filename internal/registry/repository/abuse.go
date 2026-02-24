package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
)

// AbuseReportRepository provides CRUD operations for abuse reports.
type AbuseReportRepository struct {
	db *pgxpool.Pool
}

// NewAbuseReportRepository creates a new AbuseReportRepository.
func NewAbuseReportRepository(db *pgxpool.Pool) *AbuseReportRepository {
	return &AbuseReportRepository{db: db}
}

// Create inserts a new abuse report.
func (r *AbuseReportRepository) Create(ctx context.Context, report *model.AbuseReport) error {
	report.ID = uuid.New()
	report.CreatedAt = time.Now().UTC()
	report.Status = model.AbuseStatusOpen

	query := `
		INSERT INTO abuse_reports (id, agent_id, reporter_user_id, reason, details, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.Exec(ctx, query,
		report.ID, report.AgentID, report.ReporterUserID,
		report.Reason, report.Details, report.Status, report.CreatedAt,
	)
	return err
}

// GetByID retrieves an abuse report by ID.
func (r *AbuseReportRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AbuseReport, error) {
	query := `SELECT id, agent_id, reporter_user_id, reason, details, status,
	                 resolution_note, created_at, resolved_at, resolved_by
	          FROM abuse_reports WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)
	return r.scanRow(row)
}

// List returns paginated abuse reports, optionally filtered by status.
func (r *AbuseReportRepository) List(ctx context.Context, status string, limit, offset int) ([]*model.AbuseReport, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, agent_id, reporter_user_id, reason, details, status,
	                 resolution_note, created_at, resolved_at, resolved_by
	          FROM abuse_reports
	          WHERE ($1 = '' OR status = $1)
	          ORDER BY created_at DESC
	          LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*model.AbuseReport
	for rows.Next() {
		rpt, err := r.scanRows(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rpt)
	}
	return reports, rows.Err()
}

// CountByAgentAndReporter counts open reports for a given agent by a given user.
func (r *AbuseReportRepository) CountByAgentAndReporter(ctx context.Context, agentID, userID uuid.UUID) (int, error) {
	var count int
	q := `SELECT COUNT(*) FROM abuse_reports
	      WHERE agent_id = $1 AND reporter_user_id = $2 AND status = 'open'`
	err := r.db.QueryRow(ctx, q, agentID, userID).Scan(&count)
	return count, err
}

// Resolve updates the resolution fields of an abuse report.
func (r *AbuseReportRepository) Resolve(ctx context.Context, id uuid.UUID, status model.AbuseReportStatus, note string, resolvedBy uuid.UUID) error {
	now := time.Now().UTC()
	query := `UPDATE abuse_reports SET status = $2, resolution_note = $3, resolved_at = $4, resolved_by = $5
	          WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id, status, note, now, resolvedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AbuseReportRepository) scanRow(row pgx.Row) (*model.AbuseReport, error) {
	var rpt model.AbuseReport
	err := row.Scan(
		&rpt.ID, &rpt.AgentID, &rpt.ReporterUserID,
		&rpt.Reason, &rpt.Details, &rpt.Status,
		&rpt.ResolutionNote, &rpt.CreatedAt,
		&rpt.ResolvedAt, &rpt.ResolvedBy,
	)
	if err != nil {
		return nil, err
	}
	return &rpt, nil
}

func (r *AbuseReportRepository) scanRows(rows pgx.Rows) (*model.AbuseReport, error) {
	var rpt model.AbuseReport
	err := rows.Scan(
		&rpt.ID, &rpt.AgentID, &rpt.ReporterUserID,
		&rpt.Reason, &rpt.Details, &rpt.Status,
		&rpt.ResolutionNote, &rpt.CreatedAt,
		&rpt.ResolvedAt, &rpt.ResolvedBy,
	)
	if err != nil {
		return nil, err
	}
	return &rpt, nil
}
