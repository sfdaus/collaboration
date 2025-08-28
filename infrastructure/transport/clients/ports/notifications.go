package ports

import "context"

type Notification interface {
	SendNotification(ctx context.Context, payload CreateNotification) error
}

type CreateNotification struct {
	UserID        string
	Type          string
	ReferenceType string
	ReferenceID   string
	Title         string
	Message       string
	ActionUrl     string
	Priority      string

	Headers map[string]string
}
