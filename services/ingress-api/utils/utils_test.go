package utils

import "testing"

func TestNetIdToString(t *testing.T) {
	for _, tc := range []struct {
		input []byte
		want  string
	}{
		{[]byte{0, 0, 0x13}, "000013"}, {[]byte{0xab, 0xcd, 0xef}, "ABCDEF"}, {[]byte{0, 0, 0}, "000000"},
	} {
		if got := NetIdToString(tc.input); got != tc.want {
			t.Errorf("NetIdToString(%x) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
