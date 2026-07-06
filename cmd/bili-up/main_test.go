package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintLoginQRCode(t *testing.T) {
	const url = "https://example.test/login?qrcode_key=test"

	var out bytes.Buffer
	printLoginQRCode(url, &out)

	got := out.String()
	if !strings.Contains(got, "Scan this QR code with Bilibili app:") {
		t.Fatalf("output missing scan prompt: %q", got)
	}
	if !strings.Contains(got, url) {
		t.Fatalf("output missing fallback url: %q", got)
	}
	if strings.Count(got, "\n") < 4 {
		t.Fatalf("output too short to include qr code: %q", got)
	}
}
