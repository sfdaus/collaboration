package utils

const (
	// Application Status
	PENDING_APPLICATION_STATUS   = "PENDING"
	ACCEPTED_APPLICATION_STATUS  = "ACCEPTED"
	REJECTED_APPLICATION_STATUS  = "REJECTED"
	CANCELLED_APPLICATION_STATUS = "CANCELLED"
	EXPIRED_APPLICATION_STATUS   = "EXPIRED"

	// Collaboration Status
	ACTIVE_COLLABORATION_STATUS = "ACTIVE"
	LEFT_COLLABORATION_STATUS   = "LEFT"

	// Notification Type
	THREAD_SELF_APPLICATION_NOTIFICATION_TYPE      = "THREAD_COLLAB_APPLICATION_SELF"
	THREAD_INITIATOR_APPLICATION_NOTIFICATION_TYPE = "THREAD_COLLAB_APPLICATION_INITIATOR"
	THREAD_APPLICATION_REJECT_NOTIFICATION         = "THREAD_COLLAB_REJECTED"
	THREAD_APPLICATION_APPROVE_NOTIFICATION        = "THREAD_COLLAB_APPROVED"

	// Notification Reference Type
	THREAD_APPLICATION_NOTIFICATION_REFERENCE_TYPE = "THREAD_APPLICATION"
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

var CollaborationInitiatorNotificationTitle = map[string]string{
	"THREAD_APPLICATION_TITLE":   `tertarik dengan role <role> untuk proyek "<thread_title>"`,
	"APPLICATION_REJECTED_TITLE": `menolak ajakan kolaborasi kamu untuk proyek "<thread_title>"`,
	"APPLICATION_APPROVED_TITLE": `menerima ajakan kolaborasi kamu untuk proyek "<thread_title>"`,
}

var CollaborationInitiatorNotificationMessage = map[string]string{
	"THREAD_APPLICATION_MESSAGE": `Aplikasi yang kamu ajukan diterima!`,
}

var CollaborationSelfNotificationTitle = map[string]string{
	"THREAD_APPLICATION_TITLE": "Aplikasi terkirim • <role> — <thread_title>",
}

var NotificationPriority = map[string]int32{
	"low":    1,
	"medium": 2,
	"high":   3,
}

var CollaborationNotificationPriority = map[string]string{
	"THREAD_COLLAB_APPLICATION_SELF":      "medium",
	"THREAD_COLLAB_APPLICATION_INITIATOR": "high",
	"THREAD_COLLAB_REJECTED":              "high",
	"THREAD_COLLAB_APPROVED":              "high",
}
