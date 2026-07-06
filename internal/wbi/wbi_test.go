package wbi

import "testing"

func TestSignMatchesKnownBilibiliExample(t *testing.T) {
	signer := NewSigner("7cd084941338484aae1ad9425b84077c", "4932caff0ff746eab6f01bf08b70ac45")
	signed := signer.Sign(map[string]string{
		"foo": "114",
		"bar": "514",
		"zab": "1919810",
	}, 1702204169)

	if signed["wts"] != "1702204169" {
		t.Fatalf("wts = %q", signed["wts"])
	}
	if signed["w_rid"] != "8f6f2b5b3d485fe1886cec6a0be8c5d4" {
		t.Fatalf("w_rid = %q", signed["w_rid"])
	}
}
