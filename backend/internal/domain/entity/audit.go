package entity

import "time"

type AuditEntry struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Action     string         `json:"action"`
	ActorID    string         `json:"actor_id"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  time.Time      `json:"created_at"`
}
