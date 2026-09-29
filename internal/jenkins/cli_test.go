package jenkins

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIUTFUsesJavaModifiedUTF8(t *testing.T) {
	for _, test := range []struct {
		value string
		want  []byte
	}{
		{"A", []byte{0, 1, 'A'}},
		{"\x00", []byte{0, 2, 0xc0, 0x80}},
		{"😀", []byte{0, 6, 0xed, 0xa0, 0xbd, 0xed, 0xb8, 0x80}},
	} {
		got, err := cliUTF(test.value)
		if err != nil || !bytes.Equal(got, test.want) {
			t.Errorf("%q: %v %x, want %x", test.value, err, got, test.want)
		}
	}
	if _, err := cliUTF(strings.Repeat("x", 65536)); err == nil {
		t.Error("accepted an argument beyond the protocol limit")
	}
}
