package cmd

import "testing"

func TestBuildLogPredicate(t *testing.T) {
	tests := []struct {
		name        string
		bundleID    string
		processName string
		filter      string
		want        string
	}{
		{
			name:        "bundle id only",
			bundleID:    "com.example.MyApp",
			processName: "",
			filter:      "",
			want:        `(subsystem == "com.example.MyApp")`,
		},
		{
			name:        "with process name and filter",
			bundleID:    "com.example.MyApp",
			processName: "MyApp",
			filter:      "error",
			want:        `(subsystem == "com.example.MyApp" OR process == "MyApp" OR senderImagePath ENDSWITH[c] "/MyApp") AND eventMessage CONTAINS "error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLogPredicate(tt.bundleID, tt.processName, tt.filter)
			if got != tt.want {
				t.Errorf("buildLogPredicate(%q, %q, %q) = %q, want %q", tt.bundleID, tt.processName, tt.filter, got, tt.want)
			}
		})
	}
}
