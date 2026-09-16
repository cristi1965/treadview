package models

import "time"

// AuditEvent links immutable request records and separately hashes their final outcomes.
type AuditEvent struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time  `gorm:"index;not null" json:"createdAt"`
	CompletedAt *time.Time `gorm:"index" json:"completedAt,omitempty"`
	RequestID   string     `gorm:"size:80;index;not null;uniqueIndex:ux_audit_request_v2,where:hash_version = 2" json:"requestId"`
	Actor       string     `gorm:"size:40;not null" json:"actor"`
	Action      string     `gorm:"size:80;index;not null" json:"action"`
	Target      string     `gorm:"size:240;not null" json:"target"`
	Status      string     `gorm:"size:32;index;not null;default:''" json:"status"`
	Result      string     `gorm:"size:32;index;not null;default:''" json:"result,omitempty"`
	HTTPStatus  int        `json:"httpStatus"`
	PayloadHash string     `gorm:"size:64" json:"payloadHash,omitempty"`
	OutcomeHash string     `gorm:"size:64" json:"outcomeHash,omitempty"`
	HashVersion int        `gorm:"not null;default:1;index" json:"hashVersion"`
	PrevHash    string     `gorm:"size:64" json:"prevHash"`
	Hash        string     `gorm:"size:64;uniqueIndex;not null" json:"hash"`
}
