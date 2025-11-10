package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPR_IsMerged(t *testing.T) {
	tests := []struct {
		name     string
		pr       *PR
		expected bool
	}{
		{
			name:     "nil PR returns false",
			pr:       nil,
			expected: false,
		},
		{
			name: "merged state returns true",
			pr: &PR{
				PRNumber: 123,
				State:    "merged",
			},
			expected: true,
		},
		{
			name: "open state returns false",
			pr: &PR{
				PRNumber: 123,
				State:    "open",
			},
			expected: false,
		},
		{
			name: "draft state returns false",
			pr: &PR{
				PRNumber: 123,
				State:    "draft",
			},
			expected: false,
		},
		{
			name: "closed state returns false",
			pr: &PR{
				PRNumber: 123,
				State:    "closed",
			},
			expected: false,
		},
		{
			name: "empty state returns false",
			pr: &PR{
				PRNumber: 123,
				State:    "",
			},
			expected: false,
		},
		{
			name: "unknown state returns false",
			pr: &PR{
				PRNumber: 123,
				State:    "unknown",
			},
			expected: false,
		},
		{
			name: "case-sensitive: MERGED returns false",
			pr: &PR{
				PRNumber: 123,
				State:    "MERGED",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pr.IsMerged()
			assert.Equal(t, tt.expected, result)
		})
	}
}

