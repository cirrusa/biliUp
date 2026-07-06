package tasks

import (
	"context"
	"errors"
	"testing"

	"bilitool-go/internal/bili"
	"bilitool-go/internal/config"
	"bilitool-go/internal/cookie"
	"bilitool-go/internal/wbi"
)

func TestNeededCoinsRespectsAlreadyDonatedBalanceAndProtection(t *testing.T) {
	got := neededCoins(5, 20, 4.0, 1)
	if got != 3 {
		t.Fatalf("neededCoins = %d, want 3", got)
	}

	got = neededCoins(5, 50, 10.0, 0)
	if got != 0 {
		t.Fatalf("completed neededCoins = %d, want 0", got)
	}

	got = neededCoins(5, 0, 2.0, 2)
	if got != 0 {
		t.Fatalf("protected neededCoins = %d, want 0", got)
	}
}

func TestShouldDonateSkipsWhenSaveCoinsForLv6(t *testing.T) {
	if shouldDonate(true, 6) {
		t.Fatal("expected lv6 account to skip donating when saveCoinsWhenLv6 is enabled")
	}
	if !shouldDonate(false, 6) {
		t.Fatal("expected lv6 account to donate when saveCoinsWhenLv6 is disabled")
	}
	if !shouldDonate(true, 5) {
		t.Fatal("expected non-lv6 account to donate")
	}
}

func TestPickVideoFallsBackToRankingWhenSupportUPFails(t *testing.T) {
	ck, err := cookie.Parse("DedeUserID=42; SESSDATA=sess; bili_jct=csrf")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeBili{
		upErr:   errors.New("up failed"),
		ranking: []bili.Video{{AID: 9, BVID: "BV9", CID: 99, Title: "ranking"}},
	}
	r := Runner{
		Bili: fake,
		Config: config.Config{
			Task: config.TaskConfig{SupportUpIDs: []int64{123}},
		},
	}

	video, err := r.pickVideo(context.Background(), ck, wbi.Signer{})
	if err != nil {
		t.Fatal(err)
	}

	if video.AID != 9 || fake.searchCalls != 1 || fake.rankingCalls != 1 {
		t.Fatalf("unexpected video/fallback: video=%+v searchCalls=%d rankingCalls=%d", video, fake.searchCalls, fake.rankingCalls)
	}
}

type fakeBili struct {
	upErr        error
	upVideos     []bili.Video
	ranking      []bili.Video
	searchCalls  int
	rankingCalls int
}

func (f *fakeBili) SearchUPVideos(ctx context.Context, ck *cookie.Cookie, signer wbi.Signer, mid int64, page int) (bili.SearchUPVideosData, error) {
	f.searchCalls++
	if f.upErr != nil {
		return bili.SearchUPVideosData{}, f.upErr
	}
	return bili.SearchUPVideosData{Page: bili.Page{Count: len(f.upVideos)}, List: bili.UPVideoList{VList: f.upVideos}}, nil
}

func (f *fakeBili) Ranking(ctx context.Context) ([]bili.Video, error) {
	f.rankingCalls++
	return f.ranking, nil
}
