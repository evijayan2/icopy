package icopy

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGetDestinationPath(t *testing.T) {
	tm := time.Date(2023, 10, 25, 12, 0, 0, 0, time.UTC)
	destDir := "/tmp/dest"

	tests := []struct {
		name     string
		dateFmt  string
		expected string
	}{
		{
			name:     "Format DATE",
			dateFmt:  "DATE",
			expected: filepath.Join(destDir, "2023-10-25"),
		},
		{
			name:     "Format YEAR-MONTH",
			dateFmt:  "YEAR-MONTH",
			expected: filepath.Join(destDir, "2023/10"),
		},
		{
			name:     "Format NOF (default)",
			dateFmt:  "NOF",
			expected: filepath.Join(destDir, ""),
		},
		{
			name:     "Unknown Format",
			dateFmt:  "UNKNOWN",
			expected: filepath.Join(destDir, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDestinationPath(tm, destDir, tt.dateFmt)
			if result != tt.expected {
				t.Errorf("getDestinationPath(%s) = %s; want %s", tt.dateFmt, result, tt.expected)
			}
		})
	}
}
