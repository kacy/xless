package cmd

import "testing"

func TestBuildLogPredicate(t *testing.T) {
	tests := []struct {
		name     string
		bundleID string
		filter   string
		want     string
	}{
		{
			name:     "bundle id only",
			bundleID: "com.example.MyApp",
			filter:   "",
			want:     `subsystem == "com.example.MyApp"`,
		},
		{
			name:     "with filter",
			bundleID: "com.example.MyApp",
			filter:   "error",
			want:     `subsystem == "com.example.MyApp" AND eventMessage CONTAINS "error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLogPredicate(tt.bundleID, tt.filter)
			if got != tt.want {
				t.Errorf("buildLogPredicate(%q, %q) = %q, want %q", tt.bundleID, tt.filter, got, tt.want)
			}
		})
	}
}
