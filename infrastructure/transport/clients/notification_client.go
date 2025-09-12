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
	hdr := http.Header{}
	for k, v := range payload.Headers { // <- forward custom headers if any
		hdr.Set(k, v)
	}

	_, err := n.c.Do(ctx, httpclient.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/notifications",
		Header: hdr,
		BodyJSON: map[string]any{
			"user_id":        payload.UserID,
			"type":           payload.Type,
			"reference_type": payload.ReferenceType,
			"reference_id":   payload.ReferenceID,
			"title":          payload.Title,
			"message":        payload.Message,
			"action_url":     payload.ActionUrl,
			"priority":       payload.Priority,
		},
	})
	return err
}
