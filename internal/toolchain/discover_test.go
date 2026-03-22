package toolchain

import "testing"

func TestParseSwiftVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard output",
			input: "swift-driver version: 1.87.3 Apple Swift version 5.9.2 (swiftlang-5.9.2.2.56 clang-1500.1.0.2.5)\nTarget: arm64-apple-macosx14.0",
			want:  "5.9.2",
		},
		{
			name:  "swift 6",
			input: "Apple Swift version 6.2.4 (swiftlang-6.2.4.6.7 clang-1700.3.7.6)\nTarget: arm64-apple-macosx15.0",
			want:  "6.2.4",
		},
		{
			name:  "single line",
			input: "Apple Swift version 5.10",
			want:  "5.10",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "no version marker",
			input: "something unexpected",
			want:  "something unexpected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSwiftVersion(tt.input)
			if got != tt.want {
				t.Errorf("parseSwiftVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseXcodeVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard output",
			input: "Xcode 15.1\nBuild version 15C65",
			want:  "Xcode 15.1",
		},
		{
			name:  "single line",
			input: "Xcode 26.3",
			want:  "Xcode 26.3",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseXcodeVersion(tt.input)
			if got != tt.want {
				t.Errorf("parseXcodeVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGoarchToApple(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{"arm64", "arm64"},
		{"amd64", "x86_64"},
		{"386", "386"},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			got := goarchToApple(tt.goarch)
			if got != tt.want {
				t.Errorf("goarchToApple(%q) = %q, want %q", tt.goarch, got, tt.want)
			}
		})
	}
}
