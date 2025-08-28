package http

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/labstack/echo/v4"
	"net/http"
	"prakarsa-app/delivery/middleware"
	"prakarsa-app/domain"
	"prakarsa-app/transport/request"
	"prakarsa-app/utils"
)

type CollaborationHandler struct {
	CollaborationUC domain.CollaborationUsecase
}

// NewCollaborationHandler will initialize the todo resources endpoint
func NewCollaborationHandler(e *echo.Echo, middleware *middleware.Middleware, referenceUC domain.CollaborationUsecase) {
	handler := &CollaborationHandler{
		CollaborationUC: referenceUC,
	}

	apiV1 := e.Group("/api/v1")
	apiV1.POST("/collaborations/threads/:threadID/apply", handler.ThreadCollaborationApply)
	// TODO : apiV1.POST("/collaborations/threads/:threadID/reject", handler.RejectThreadCollaboration)
	// TODO : apiV1.POST("/collaborations/threads/:threadID/accept", handler.AcceptThreadCollaboration)
}

func (h *CollaborationHandler) ThreadCollaborationApply(c echo.Context) error {
	ctx := c.Request().Context()
	var req request.ThreadCollaborationApplyReq

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, utils.NewUnprocessableEntityError(err.Error()))
	}

	req.UserID = c.Request().Header.Get("x-user-id")

	if err := req.Validate(); err != nil {
		errVal := err.(validation.Errors)
		return c.JSON(http.StatusBadRequest, utils.NewInvalidInputError(errVal))
	}

	if res, err := h.CollaborationUC.ThreadCollaborationApply(ctx, &req); err != nil {
		return c.JSON(utils.ParseHttpErrorToBasicResponse(err, "Apply thread collaboration failed"))
	} else {
		return c.JSON(http.StatusCreated, map[string]interface{}{
			"message": "Apply collaboration successful",
			"data":    res,
		})
	}
}
