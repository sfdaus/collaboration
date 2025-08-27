package utils

const (
	PENDING_APPLICATION_STATUS   = "PENDING"
	ACCEPTED_APPLICATION_STATUS  = "ACCEPTED"
	REJECTED_APPLICATION_STATUS  = "REJECTED"
	CANCELLED_APPLICATION_STATUS = "CANCELLED"
	EXPIRED_APPLICATION_STATUS   = "EXPIRED"
)

type ResponseStatus struct {
	Success string
	Failed  string
	Error   string
}

var Status = ResponseStatus{
	Success: "success",
	Failed:  "failed",
	Error:   "error",
}
