package acpclient

// Request represents an inbound message to be sent to an ACP agent.
type Request struct {
	ScopeKey string // e.g. "slack:C123:1234567890.123456"
	Message  string
}

// Response represents the collected output from an ACP agent.
type Response struct {
	Text string
}
