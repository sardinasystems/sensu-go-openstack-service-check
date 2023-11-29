package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
}

func TestReasonMatch(t *testing.T) {
	testCases := []struct {
		name     string
		reason   string
		patterns []string
		expected bool
	}{
		{"nil", "", nil, false},
		{"match1", " bar ", []string{"bar", "foo.*"}, true},
		{"match2", "foo_quux", []string{"bar", "foo.*"}, true},
		{"not-match", "manual", []string{"bar", "foo.*"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			obtained := reasonMatch(tc.reason, tc.patterns)
			assert.Equal(t, tc.expected, obtained)
		})
	}
}
