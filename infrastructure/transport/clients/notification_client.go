package clients

import (
	"context"
	"net/http"
	"prakarsa-app/infrastructure/transport/clients/ports"
	"prakarsa-app/infrastructure/transport/httpclient"
)

type NotificationClient struct {
	c *httpclient.Client
}

func NewNotificationClient(c *httpclient.Client) *NotificationClient {
	return &NotificationClient{c: c}
}

func (n *NotificationClient) SendNotification(ctx context.Context, payload ports.CreateNotification) error {
	_, err := n.c.Do(ctx, httpclient.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/general/notifications",
		BodyJSON: map[string]any{
			"user_id":        payload.UserID,
			"type":           payload.Type,
			"reference_type": payload.ReferenceType,
			"reference_id":   payload.ReferenceID,
			"source_user_id": payload.SourceUserID,
			"title":          payload.Title,
			"message":        payload.Message,
			"action_url":     payload.ActionUrl,
			"priority":       payload.Priority,
		},
	})
	return err
}
