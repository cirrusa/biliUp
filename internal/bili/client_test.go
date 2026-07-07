package bili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"bili-up/internal/cookie"
	"bili-up/internal/wbi"
)

func TestPollQRCodeExtractsCookieFromSetCookieHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/passport-login/web/qrcode/poll" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		http.SetCookie(w, &http.Cookie{Name: "DedeUserID", Value: "42"})
		http.SetCookie(w, &http.Cookie{Name: "SESSDATA", Value: "sess"})
		http.SetCookie(w, &http.Cookie{Name: "bili_jct", Value: "csrf"})
		_ = json.NewEncoder(w).Encode(Response[QRCodePoll]{Code: 0, Data: QRCodePoll{Code: 0, Message: "ok"}})
	}))
	defer server.Close()

	client := NewClient(Options{PassportBaseURL: server.URL})
	ck, done, err := client.PollQRCode(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("expected poll to be done")
	}
	if ck.UserID() != "42" || ck.SessData() != "sess" || ck.BiliJct() != "csrf" {
		t.Fatalf("unexpected cookie: %#v", ck.Items)
	}
}

func TestSearchUPVideosSendsSignedWBIQuery(t *testing.T) {
	var rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(Response[SearchUPVideosData]{
			Code: 0,
			Data: SearchUPVideosData{
				Page: Page{Count: 1},
				List: UPVideoList{VList: []Video{{AID: 1, BVID: "BV1", Title: "v"}}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(Options{APIBaseURL: server.URL})
	ck, err := cookie.Parse("DedeUserID=42; SESSDATA=sess; bili_jct=csrf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SearchUPVideos(context.Background(), ck, wbi.NewSigner("7cd084941338484aae1ad9425b84077c", "4932caff0ff746eab6f01bf08b70ac45"), 123, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rawQuery, "w_rid=") || !strings.Contains(rawQuery, "wts=") || !strings.Contains(rawQuery, "mid=123") {
		t.Fatalf("query was not signed as expected: %s", rawQuery)
	}
}

func TestWatchShareAndAddCoinSendExpectedForms(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		seen = append(seen, r.URL.Path)
		if !strings.Contains(r.Header.Get("Cookie"), "bili_jct=csrf") {
			t.Fatalf("missing cookie on %s: %s", r.URL.Path, r.Header.Get("Cookie"))
		}
		switch r.URL.Path {
		case "/x/click-interface/web/heartbeat":
			assertForm(t, r.Form, "aid", "11")
			assertForm(t, r.Form, "bvid", "BV11")
			assertForm(t, r.Form, "cid", "22")
			assertForm(t, r.Form, "mid", "42")
			assertForm(t, r.Form, "csrf", "csrf")
			if r.Header.Get("Referer") != "https://www.bilibili.com/" {
				t.Fatalf("watch referer = %q", r.Header.Get("Referer"))
			}
		case "/x/web-interface/share/add":
			assertForm(t, r.Form, "aid", "11")
			assertForm(t, r.Form, "csrf", "csrf")
			assertForm(t, r.Form, "source", "web_normal")
		case "/x/web-interface/coin/add":
			assertForm(t, r.Form, "aid", "11")
			assertForm(t, r.Form, "multiply", "1")
			assertForm(t, r.Form, "select_like", "1")
			assertForm(t, r.Form, "csrf", "csrf")
			if r.Header.Get("Referer") != "https://www.bilibili.com/video/BV11/" {
				t.Fatalf("coin referer = %q", r.Header.Get("Referer"))
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Response[Empty]{Code: 0})
	}))
	defer server.Close()

	client := NewClient(Options{APIBaseURL: server.URL})
	ck, err := cookie.Parse("DedeUserID=42; SESSDATA=sess; bili_jct=csrf")
	if err != nil {
		t.Fatal(err)
	}
	video := Video{AID: 11, BVID: "BV11", CID: 22, Duration: 15}
	if err := client.WatchVideo(context.Background(), ck, video); err != nil {
		t.Fatal(err)
	}
	if err := client.ShareVideo(context.Background(), ck, video); err != nil {
		t.Fatal(err)
	}
	if err := client.AddCoin(context.Background(), ck, video, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(seen, ","); got != "/x/click-interface/web/heartbeat,/x/web-interface/share/add,/x/web-interface/coin/add" {
		t.Fatalf("unexpected call order: %s", got)
	}
}

func TestLevelInfoUnmarshalParsesNextExp(t *testing.T) {
	var level LevelInfo
	if err := json.Unmarshal([]byte(`{"current_level":5,"current_exp":100,"next_exp":200}`), &level); err != nil {
		t.Fatal(err)
	}
	if level.CurrentLevel != 5 || level.CurrentExp != 100 || level.NextExp != 200 {
		t.Fatalf("unexpected level info: %+v", level)
	}

	if err := json.Unmarshal([]byte(`{"current_level":6,"current_exp":28800,"next_exp":"--"}`), &level); err != nil {
		t.Fatal(err)
	}
	if level.CurrentLevel != 6 || level.CurrentExp != 28800 || level.NextExp != 0 {
		t.Fatalf("unexpected lv6 level info: %+v", level)
	}
}

func assertForm(t *testing.T, form url.Values, key, want string) {
	t.Helper()
	if got := form.Get(key); got != want {
		t.Fatalf("form %s = %q, want %q", key, got, want)
	}
}
