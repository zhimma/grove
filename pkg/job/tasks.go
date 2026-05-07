package job

const TaskEcho = "echo"

type EchoPayload struct {
	Message     string `json:"message"`
	RequestedBy string `json:"requested_by,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
}
