package executor

// MessageRequest represents an incoming message to be processed by the agent
type MessageRequest struct {
	UserID    string // Unique identifier for the user
	SessionID string // Unique identifier for the conversation session
	Message   string // The user's message text
}

// MessageResponse represents the agent's response
type MessageResponse struct {
	Text string // The agent's response text
}
