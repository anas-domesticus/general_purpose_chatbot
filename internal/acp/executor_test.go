package acpclient

import (
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
)

func TestFormatResponse(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		stopReason acp.StopReason
		want       string
	}{
		{
			name:       "text present with end_turn",
			text:       "hello",
			stopReason: acp.StopReasonEndTurn,
			want:       "hello",
		},
		{
			name:       "text present with max_tokens — text takes priority",
			text:       "hello",
			stopReason: acp.StopReasonMaxTokens,
			want:       "hello",
		},
		{
			name:       "empty text with end_turn — normal empty response",
			text:       "",
			stopReason: acp.StopReasonEndTurn,
			want:       "",
		},
		{
			name:       "empty text with max_tokens — synthetic message",
			text:       "",
			stopReason: acp.StopReasonMaxTokens,
			want:       "[agent stopped: max_tokens]",
		},
		{
			name:       "empty text with refusal",
			text:       "",
			stopReason: acp.StopReasonRefusal,
			want:       "[agent stopped: refusal]",
		},
		{
			name:       "empty text with canceled", //nolint:misspell // SDK uses "cancelled"
			text:       "",
			stopReason: acp.StopReasonCancelled,
			want:       "[agent stopped: cancelled]", //nolint:misspell // SDK spelling
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatResponse(tt.text, tt.stopReason)
			assert.Equal(t, tt.want, got)
		})
	}
}
