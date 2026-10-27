package tlsprofile

import (
	"crypto/tls"
	"testing"
)

func TestTlsVersionString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input uint16
		want  string
	}{
		{tls.VersionTLS10, "1.0"},
		{tls.VersionTLS11, "1.1"},
		{tls.VersionTLS12, "1.2"},
		{tls.VersionTLS13, "1.3"},
		{0, ""},
	}
	for _, tc := range cases {
		got := tlsVersionString(tc.input)
		if got != tc.want {
			t.Errorf("tlsVersionString(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
