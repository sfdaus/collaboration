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
