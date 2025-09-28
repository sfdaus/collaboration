package response

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
}
