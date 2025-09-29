package request

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

// Apply Thread Collaboration request body
type ThreadCollaborationApplyReq struct {
	ThreadID      string `param:"threadID"`
	PartnerTypeID string `json:"partner_type_id"`
	Message       string `json:"message"`
	UserID        string
}

func (request ThreadCollaborationApplyReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.ThreadID, validation.Required),
		validation.Field(&request.PartnerTypeID, validation.Required),
		validation.Field(&request.UserID, validation.Required),
	)
}

// Reject Thread Collaboration request body
type RejectThreadCollaborationReq struct {
	ApplicationCollaborationID string `param:"applicationID"`
	Message                    string `json:"message"`
	UserID                     string
}

func (request RejectThreadCollaborationReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.ApplicationCollaborationID, validation.Required),
		validation.Field(&request.UserID, validation.Required),
	)
}

// Approve Thread Collaboration request body
type ApproveThreadCollaborationReq struct {
	ApplicationCollaborationID string `param:"applicationID"`
	UserID                     string
}

func (request ApproveThreadCollaborationReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.ApplicationCollaborationID, validation.Required),
		validation.Field(&request.UserID, validation.Required),
	)
}

// My Thread Collaboration request body
type MyThreadCollaborationReq struct {
	PerPage int64  `query:"per_page"`
	Page    int64  `query:"page"`
	Status  string `query:"status"`
	UserID  string
}

func (request MyThreadCollaborationReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.UserID, validation.Required),
		validation.Field(&request.Status,
			validation.In("PENDING", "ACCEPTED", "REJECTED"),
		),
	)
}

// My Thread Collaboration Requests request body
type MyThreadCollaborationRequestsReq struct {
	PerPage int64 `query:"per_page"`
	Page    int64 `query:"page"`
	UserID  string
}

func (request MyThreadCollaborationRequestsReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.UserID, validation.Required),
	)
}

// Accepted Thread Collaboration Requests request body
type AcceptedThreadCollaborationRequestsReq struct {
	PerPage int64 `query:"per_page"`
	Page    int64 `query:"page"`
	UserID  string
}

func (request AcceptedThreadCollaborationRequestsReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.UserID, validation.Required),
	)
}

// Cancel Thread Collaboration request body
type CancelThreadCollaborationReq struct {
	ApplicationCollaborationID string `param:"applicationID"`
	UserRelation               string `json:"user_relation"`
	Message                    string `json:"message"`
	UserID                     string
}

func (request CancelThreadCollaborationReq) Validate() error {
	return validation.ValidateStruct(
		&request,
		validation.Field(&request.ApplicationCollaborationID, validation.Required),
		validation.Field(&request.UserID, validation.Required),
		validation.Field(&request.UserRelation,
			validation.Required,
			validation.In("INITIATOR", "COLLABORATOR"),
		),
	)
}
