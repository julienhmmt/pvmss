package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Quota request statuses. A request leaves "pending" exactly once.
const (
	QuotaRequestPending  = "pending"
	QuotaRequestApproved = "approved"
	QuotaRequestDenied   = "denied"
)

// QuotaRequest is one quota_requests row: a user's ask for a higher VM cap
// on their own pool. CurrentQuota is the snapshot taken when it was filed.
type QuotaRequest struct {
	ID             int64
	Requester      string
	Cluster        string
	Pool           string
	CurrentQuota   int
	RequestedQuota int
	Reason         string
	Status         string
	DecidedBy      string
	DecidedAt      string
	CreatedAt      string
}

const quotaRequestColumns = `id, requester, cluster, pool, current_quota, requested_quota,
	reason, status, COALESCE(decided_by, ''), COALESCE(decided_at, ''), created_at`

// CreateQuotaRequest files a pending request. It returns ErrDuplicate when the
// requester already has a pending request for the same cluster and pool.
func (s *Store) CreateQuotaRequest(ctx context.Context, row QuotaRequest) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO quota_requests
			(requester, cluster, pool, current_quota, requested_quota, reason, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		[]any{
			row.Requester, row.Cluster, row.Pool, row.CurrentQuota, row.RequestedQuota,
			row.Reason, QuotaRequestPending, time.Now().UTC().Format(time.RFC3339),
		},
	)
}

// PendingQuotaRequest returns the requester's open request on the pool, or
// sql.ErrNoRows when there is none.
func (s *Store) PendingQuotaRequest(ctx context.Context, requester, cluster, pool string) (QuotaRequest, error) {
	return s.scanQuotaRequest(s.db.QueryRowContext(ctx,
		`SELECT `+quotaRequestColumns+` FROM quota_requests
		 WHERE requester = ? AND cluster = ? AND pool = ? AND status = ?`,
		requester, cluster, pool, QuotaRequestPending))
}

// LatestQuotaRequest returns the requester's most recent request on the pool
// whatever its status, or sql.ErrNoRows when they never asked.
func (s *Store) LatestQuotaRequest(ctx context.Context, requester, cluster, pool string) (QuotaRequest, error) {
	return s.scanQuotaRequest(s.db.QueryRowContext(ctx,
		`SELECT `+quotaRequestColumns+` FROM quota_requests
		 WHERE requester = ? AND cluster = ? AND pool = ? ORDER BY id DESC LIMIT 1`,
		requester, cluster, pool))
}

// QuotaRequestByID returns one request, or sql.ErrNoRows.
func (s *Store) QuotaRequestByID(ctx context.Context, id int64) (QuotaRequest, error) {
	return s.scanQuotaRequest(s.db.QueryRowContext(ctx,
		`SELECT `+quotaRequestColumns+` FROM quota_requests WHERE id = ?`, id))
}

// ListQuotaRequests returns requests with the given status, oldest first. An
// empty status returns every request.
func (s *Store) ListQuotaRequests(ctx context.Context, status string) ([]QuotaRequest, error) {
	query := `SELECT ` + quotaRequestColumns + ` FROM quota_requests`
	args := []any{}

	if status != "" {
		query += ` WHERE status = ?`

		args = append(args, status)
	}

	rows, err := s.db.QueryContext(ctx, query+` ORDER BY id`, args...)
	if err != nil {
		return nil, fmt.Errorf("query quota requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []QuotaRequest

	for rows.Next() {
		row, err := scanQuotaRequestRow(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota requests: %w", err)
	}

	return out, nil
}

// DecideQuotaRequest moves a pending request to status (approved or denied).
// It returns sql.ErrNoRows when the request is missing or already decided, so
// a second admin deciding the same request cannot overwrite the first.
func (s *Store) DecideQuotaRequest(ctx context.Context, id int64, status, decidedBy string) error {
	return execUpdateOne(ctx, s.db, quotaDecideSQL,
		quotaDecideArgs(id, status, decidedBy),
	)
}

const quotaDecideSQL = `UPDATE quota_requests SET status = ?, decided_by = ?, decided_at = ?
	WHERE id = ? AND status = ?`

func quotaDecideArgs(id int64, status, decidedBy string) []any {
	return []any{status, decidedBy, time.Now().UTC().Format(time.RFC3339), id, QuotaRequestPending}
}

// DecideQuotaRequestWithPolicy decides a pending request and applies the policy
// row CAS in one transaction. A lost decision race or a changed policy row rolls
// back both writes, so the request and the cap can never disagree.
func (s *Store) DecideQuotaRequestWithPolicy(ctx context.Context, id int64, status, decidedBy string, expected *PolicyRow, next PolicyRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin quota decision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, quotaDecideSQL, quotaDecideArgs(id, status, decidedBy)...)
	if err != nil {
		return fmt.Errorf("exec quota decision: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count decided rows: %w", err)
	}

	if affected == 0 {
		return sql.ErrNoRows
	}

	if err := replacePolicyRow(ctx, tx, expected, next); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit quota decision: %w", err)
	}

	return nil
}

func (s *Store) scanQuotaRequest(row *sql.Row) (QuotaRequest, error) {
	return scanQuotaRequestRow(row)
}

type quotaRequestScanner interface {
	Scan(dest ...any) error
}

func scanQuotaRequestRow(row quotaRequestScanner) (QuotaRequest, error) {
	var item QuotaRequest

	err := row.Scan(&item.ID, &item.Requester, &item.Cluster, &item.Pool, &item.CurrentQuota,
		&item.RequestedQuota, &item.Reason, &item.Status, &item.DecidedBy, &item.DecidedAt, &item.CreatedAt)
	if err != nil {
		return QuotaRequest{}, fmt.Errorf("scan quota request: %w", err)
	}

	return item, nil
}
