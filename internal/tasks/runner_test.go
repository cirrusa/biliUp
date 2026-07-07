package tasks

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"bili-up/internal/bili"
	"bili-up/internal/config"
	"bili-up/internal/cookie"
	"bili-up/internal/store"
	"bili-up/internal/wbi"
)

func TestRunAccountLogsExperienceAndUpgradeProgress(t *testing.T) {
	var logs bytes.Buffer
	client := &fakeFullBili{
		user: bili.UserInfo{
			Uname: "tester",
			Money: 7,
			LevelInfo: bili.LevelInfo{
				CurrentLevel: 5,
				CurrentExp:   100,
				NextExp:      200,
			},
		},
		reward:    bili.DailyReward{},
		ranking:   []bili.Video{{AID: 1, BVID: "BV1", CID: 2, Title: "video"}},
		donateExp: 50,
		balance:   10,
	}
	r := Runner{
		Bili: client,
		Config: config.Config{
			Task: config.TaskConfig{
				WatchVideo:    true,
				ShareVideo:    true,
				NumberOfCoins: 5,
			},
		},
		Logger: log.New(&logs, "", 0),
	}

	err := r.runAccount(context.Background(), client, store.Account{
		UID:    "42",
		Cookie: "DedeUserID=42; SESSDATA=sess; bili_jct=csrf; buvid3=buvid",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := logs.String()
	assertContains(t, got, "登录校验成功: 用户名=tester uid=42，经验+5")
	assertContains(t, got, "升级进度: 距离升级到 Lv6 还需 100 经验，按当前配置预计还需 1 天")
	assertContains(t, got, "观看任务完成: aid=1 bvid=BV1，经验+5")
	assertContains(t, got, "分享任务完成: aid=1 bvid=BV1，经验+5")
}

func TestDonateCoinsLogsProgressAndExperience(t *testing.T) {
	ck, err := cookie.Parse("DedeUserID=42; SESSDATA=sess; bili_jct=csrf; buvid3=buvid")
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	client := &fakeFullBili{
		ranking:    []bili.Video{{AID: 9, BVID: "BV9", CID: 99, Title: "coin-video"}},
		donateExp:  40,
		balance:    5,
		coinStatus: bili.ArchiveCoins{Multiply: 0},
	}
	r := Runner{
		Bili: client,
		Config: config.Config{
			Task: config.TaskConfig{
				NumberOfCoins: 5,
			},
		},
		Logger: log.New(&logs, "", 0),
	}

	if err := r.donateCoins(context.Background(), client, ck, wbi.Signer{}); err != nil {
		t.Fatal(err)
	}

	got := logs.String()
	assertContains(t, got, "投币任务进度: 今日已获得 40 点经验（已投 4/5 枚），当前硬币余额 5.0，还需投 1 枚")
	assertContains(t, got, "投币成功: aid=9 bvid=BV9 标题=\"coin-video\"，经验+10")
	assertContains(t, got, "投币任务完成: 本次成功投币 1 枚，经验+10")
	if client.addCoinCalls != 1 {
		t.Fatalf("addCoinCalls = %d, want 1", client.addCoinCalls)
	}
}

func TestEstimateUpgradeDaysMatchesLegacyBehavior(t *testing.T) {
	task := config.Default().Task

	days := estimateUpgradeDays(bili.UserInfo{
		Money: 7,
		LevelInfo: bili.LevelInfo{
			CurrentLevel: 5,
			CurrentExp:   100,
			NextExp:      200,
		},
	}, task)
	if days != 1 {
		t.Fatalf("estimateUpgradeDays = %d, want 1", days)
	}

	days = estimateUpgradeDays(bili.UserInfo{
		Money: 7,
		LevelInfo: bili.LevelInfo{
			CurrentLevel: 5,
			CurrentExp:   1000,
			NextExp:      2000,
		},
	}, task)
	if days != 37 {
		t.Fatalf("estimateUpgradeDays = %d, want 37", days)
	}
}

type fakeFullBili struct {
	user         bili.UserInfo
	reward       bili.DailyReward
	ranking      []bili.Video
	donateExp    int
	balance      float64
	coinStatus   bili.ArchiveCoins
	addCoinCalls int
}

func (f *fakeFullBili) SetCookie(context.Context, *cookie.Cookie) error {
	return nil
}

func (f *fakeFullBili) Nav(context.Context, *cookie.Cookie) (bili.UserInfo, error) {
	return f.user, nil
}

func (f *fakeFullBili) DailyReward(context.Context, *cookie.Cookie) (bili.DailyReward, error) {
	return f.reward, nil
}

func (f *fakeFullBili) WatchVideo(context.Context, *cookie.Cookie, bili.Video) error {
	return nil
}

func (f *fakeFullBili) ShareVideo(context.Context, *cookie.Cookie, bili.Video) error {
	return nil
}

func (f *fakeFullBili) CoinBalance(context.Context, *cookie.Cookie) (float64, error) {
	return f.balance, nil
}

func (f *fakeFullBili) DonateCoinExp(context.Context, *cookie.Cookie) (int, error) {
	return f.donateExp, nil
}

func (f *fakeFullBili) ArchiveCoins(context.Context, *cookie.Cookie, int64) (bili.ArchiveCoins, error) {
	return f.coinStatus, nil
}

func (f *fakeFullBili) AddCoin(context.Context, *cookie.Cookie, bili.Video, bool) error {
	f.addCoinCalls++
	return nil
}

func (f *fakeFullBili) SearchUPVideos(context.Context, *cookie.Cookie, wbi.Signer, int64, int) (bili.SearchUPVideosData, error) {
	return bili.SearchUPVideosData{}, nil
}

func (f *fakeFullBili) Ranking(context.Context) ([]bili.Video, error) {
	return f.ranking, nil
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("log missing %q:\n%s", want, got)
	}
}
