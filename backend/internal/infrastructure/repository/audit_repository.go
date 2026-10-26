package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

type AuditRepository struct {
	db *DB
}

func NewAuditRepository(db *DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) List(ctx context.Context, filter port.AuditFilter) ([]entity.AuditEntry, int, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	conn, err := r.db.TenantConn(ctx, filter.TenantID)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	where := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	addFilter := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addFilter("entity_type", filter.EntityType)
	addFilter("entity_id", filter.EntityID)
	addFilter("action", filter.Action)
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit entries: %w", err)
	}

	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id::text, tenant_id::text, entity_type, entity_id::text, action,
		       COALESCE(actor_id::text, ''), payload, created_at
		FROM audit_entries
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, limitPosition, offsetPosition)
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	entries := make([]entity.AuditEntry, 0)
	for rows.Next() {
		var entry entity.AuditEntry
		var payload []byte
		if err := rows.Scan(
			&entry.ID, &entry.TenantID, &entry.EntityType, &entry.EntityID,
			&entry.Action, &entry.ActorID, &payload, &entry.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit entry: %w", err)
		}
		if err := json.Unmarshal(payload, &entry.Payload); err != nil {
			return nil, 0, fmt.Errorf("decode audit entry payload: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit entries: %w", err)
	}
	return entries, total, nil
}

func auditActorID(ctx context.Context) (any, error) {
	actorID, ok := ctx.Value(ctxkey.ActorIDKey).(string)
	if !ok || actorID == "" {
		return nil, nil
	}
	if _, err := uuid.Parse(actorID); err != nil {
		return nil, fmt.Errorf("invalid audit actor ID: %w", err)
	}
	return actorID, nil
}

var _ port.AuditRepository = (*AuditRepository)(nil)
