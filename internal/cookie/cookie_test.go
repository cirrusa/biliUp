package cookie

import "testing"

func TestParseValidatesRequiredFields(t *testing.T) {
	ck, err := Parse("DedeUserID=42; SESSDATA=sess; bili_jct=csrf; buvid3=buvid")
	if err != nil {
		t.Fatal(err)
	}

	if ck.UserID() != "42" || ck.SessData() != "sess" || ck.BiliJct() != "csrf" || ck.Buvid3() != "buvid" {
		t.Fatalf("parsed cookie fields wrong: %#v", ck.Items)
	}
}

func TestParseRejectsMissingRequiredField(t *testing.T) {
	_, err := Parse("DedeUserID=42; SESSDATA=sess")
	if err == nil {
		t.Fatal("expected missing bili_jct error")
	}
}

func TestMergeSetCookieHeadersUpdatesAndAddsValues(t *testing.T) {
	ck, err := Parse("DedeUserID=42; SESSDATA=old; bili_jct=csrf")
	if err != nil {
		t.Fatal(err)
	}

	ck.MergeSetCookieHeaders([]string{
		"SESSDATA=new; Path=/; Domain=.bilibili.com",
		"buvid3=abc; Expires=Tue, 01 Jan 2030 00:00:00 GMT; Path=/",
	})

	if ck.SessData() != "new" {
		t.Fatalf("SESSDATA = %q", ck.SessData())
	}
	if ck.Buvid3() != "abc" {
		t.Fatalf("buvid3 = %q", ck.Buvid3())
	}
}
