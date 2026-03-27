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
			want:        `(subsystem == "com.example.MyApp" OR senderImagePath ENDSWITH[c] "/MyApp" OR senderImagePath CONTAINS[c] "/MyApp.app/") AND eventMessage CONTAINS[c] "error"`,
		},
		{
			name:        "with process name and no filter",
			bundleID:    "com.example.MyApp",
			processName: "MyApp",
			filter:      "",
			want:        `(subsystem == "com.example.MyApp" OR senderImagePath ENDSWITH[c] "/MyApp" OR senderImagePath CONTAINS[c] "/MyApp.app/")`,
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
