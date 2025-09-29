package response

import "prakarsa-app/entity"

// Thread Collaboration Apply Response
type ThreadCollaborationApplyRes struct {
	ID         string `json:"id"`
	ThreadID   string `json:"thread_id"`
	ThreadName string `json:"thread_name"`
	RoleID     string `json:"role_id"`
	RoleName   string `json:"role_name"`
	Status     string `json:"status"`
}

// My Thread Collaboration Response
type MyThreadCollaborationRes struct {
	ThreadID        string               `json:"thread_id"`
	ThreadName      string               `json:"thread_name"`
	ApplicationID   string               `json:"application_id"`
	PartnerTypeName string               `json:"partner_type_name"`
	Profile         entity.SimpleProfile `json:"profile"`
	Status          string               `json:"status"`
	CreatedAt       int64                `json:"created_at"`
}

// My Thread Collaboration Requests Response
type MyThreadCollaborationRequestsRes struct {
	ID              string               `json:"id"`
	ThreadName      string               `json:"thread_name"`
	PartnerTypeName string               `json:"partner_type_name"`
	Message         string               `json:"message"`
	Profile         entity.SimpleProfile `json:"profile"`
	CreatedAt       int64                `json:"created_at"`
}

// Accepted Thread Collaboration Requests Response
type AcceptedThreadCollaborationRequestsRes struct {
	ThreadID        string               `json:"thread_id"`
	ThreadName      string               `json:"thread_name"`
	PartnerTypeName string               `json:"partner_type_name"`
	ApplicationID   string               `json:"application_id"`
	Profile         entity.SimpleProfile `json:"profile"`
	CreatedAt       int64                `json:"created_at"`
}

// Cancel Thread Collaboration Response
type CancelThreadCollaborationRes struct {
	ApplicationID string `json:"application_id"`
}
