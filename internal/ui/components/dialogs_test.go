package components

import "testing"

func TestIsValidHHMM(t *testing.T) {
	valid := []string{"00:00", "09:05", "23:59"}
	for _, value := range valid {
		if !IsValidHHMM(value) {
			t.Errorf("IsValidHHMM(%q) = false, want true", value)
		}
	}

	invalid := []string{"", "9:05", "24:00", "12:60", "12-30", "aa:bb", "123:45"}
	for _, value := range invalid {
		if IsValidHHMM(value) {
			t.Errorf("IsValidHHMM(%q) = true, want false", value)
		}
	}
}

func TestBasenameFromURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com/releases/app.tar.gz?download=1#part": "app.tar.gz",
		"https://example.com/dir/":                                "dir",
		"plain-file.bin":                                          "plain-file.bin",
	}
	for raw, want := range tests {
		if got := BasenameFromURL(raw); got != want {
			t.Errorf("BasenameFromURL(%q) = %q, want %q", raw, got, want)
		}
	}
}
