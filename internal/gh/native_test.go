package gh

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIResponse(t *testing.T) {
	tests := []struct {
		name, response string
		wantStatus     int
		wantBody       string
		wantErr        bool
	}{
		{"success", "HTTP/2.0 200 OK\r\ncontent-type: application/json\r\n\r\n{\"ok\":true}\n", 200, "{\"ok\":true}\n", false},
		{"no content", "HTTP/2.0 204 No Content\r\n\r\n", 204, "", false},
		{"invalid", "not an HTTP response", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPIResponse([]byte(tt.response), "detail")
			assert.Equal(t, tt.wantErr, err != nil)
			if err == nil {
				assert.Equal(t, tt.wantStatus, got.Status)
				assert.Equal(t, tt.wantBody, string(got.Body))
				assert.Equal(t, "detail", got.Stderr)
			}
		})
	}
}

func TestAPIStatus(t *testing.T) {
	err := &APIError{StatusCode: 409}
	require.Error(t, err)
	assert.Equal(t, 409, APIStatus(err))
}
