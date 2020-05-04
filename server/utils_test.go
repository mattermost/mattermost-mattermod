package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	lists := [][]string{
		{},
		{"foo", "bar", "buzz"},
	}
	tests := []struct {
		name     string
		username string
		expected []bool
	}{
		{
			name:     "test",
			username: "foo",
			expected: []bool{false, true},
		},
		{
			name:     "test",
			username: "bar",
			expected: []bool{false, true},
		},
		{
			name:     "test",
			username: "baz",
			expected: []bool{false, false},
		},
	}
	for _, tt := range tests {
		for i, ll := range lists {
			t.Run(tt.name, func(t *testing.T) {
				actual := contains(ll, tt.username)
				assert.Equal(t, actual, tt.expected[i])
			})
		}
	}

}
