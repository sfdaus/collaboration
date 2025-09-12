package entity

type NotificationOutboxInsert struct {
	ID             string
	UserID         string
	Type           string
	ReferenceType  string
	ReferenceID    string
	HeadersJSON    []byte // json.Marshal(map[string]string)
	Title          string
	Message        string
	ActionURL      *string // optional, boleh nil
	Priority       string
	IdempotencyKey string
	CreatedAt      int64
	UpdatedAt      int64
}
