package ui

import "testing"

func TestParseSpeedLimit(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"500k", 500 * 1024, false},
		{"2m", 2 * 1024 * 1024, false},
		{"1.5m", 1024 * 1024 * 3 / 2, false},
		{"1g", 1024 * 1024 * 1024, false},
		{"2048", 2048, false},
		{"500K", 500 * 1024, false},
		{"2M", 2 * 1024 * 1024, false},
		{"500kb", 500 * 1024, false},
		{"500kb/s", 500 * 1024, false},
		{"  2m  ", 2 * 1024 * 1024, false},
		{"abc", 0, true},
		{"-5m", 0, true},
		{"512", 0, true}, // di bawah 1k
	}

	for _, tt := range tests {
		got, err := parseSpeedLimit(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSpeedLimit(%q) = %d, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSpeedLimit(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSpeedLimit(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSpeedInputValueRoundTrip(t *testing.T) {
	values := []int64{0, 1024, 500 * 1024, 2 * 1024 * 1024, 1024 * 1024 * 1024, 1536}

	for _, v := range values {
		s := speedInputValue(v)
		got, err := parseSpeedLimit(s)
		if err != nil {
			t.Errorf("speedInputValue(%d) = %q, parse failed: %v", v, s, err)
			continue
		}
		if got != v {
			t.Errorf("round trip %d -> %q -> %d", v, s, got)
		}
	}
}

func TestFormatSpeedLimit(t *testing.T) {
	if got := formatSpeedLimit(0); got != "unlimited" {
		t.Errorf("formatSpeedLimit(0) = %q, want unlimited", got)
	}
	if got := formatSpeedLimit(-1); got != "unlimited" {
		t.Errorf("formatSpeedLimit(-1) = %q, want unlimited", got)
	}
	if got := formatSpeedLimit(1024); got == "unlimited" {
		t.Errorf("formatSpeedLimit(1024) should not be unlimited")
	}
}
