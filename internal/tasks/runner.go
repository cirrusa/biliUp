package tasks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"bilitool-go/internal/bili"
	"bilitool-go/internal/config"
	"bilitool-go/internal/cookie"
	"bilitool-go/internal/store"
	"bilitool-go/internal/wbi"
)

type BiliClient interface {
	SearchUPVideos(context.Context, *cookie.Cookie, wbi.Signer, int64, int) (bili.SearchUPVideosData, error)
	Ranking(context.Context) ([]bili.Video, error)
}

type FullBiliClient interface {
	BiliClient
	SetCookie(context.Context, *cookie.Cookie) error
	Nav(context.Context, *cookie.Cookie) (bili.UserInfo, error)
	DailyReward(context.Context, *cookie.Cookie) (bili.DailyReward, error)
	WatchVideo(context.Context, *cookie.Cookie, bili.Video) error
	ShareVideo(context.Context, *cookie.Cookie, bili.Video) error
	CoinBalance(context.Context, *cookie.Cookie) (float64, error)
	DonateCoinExp(context.Context, *cookie.Cookie) (int, error)
	ArchiveCoins(context.Context, *cookie.Cookie, int64) (bili.ArchiveCoins, error)
	AddCoin(context.Context, *cookie.Cookie, bili.Video, bool) error
}

type Runner struct {
	Bili   BiliClient
	Store  store.Store
	Config config.Config
	Logger *log.Logger
}

func (r *Runner) Run(ctx context.Context) error {
	full, ok := r.Bili.(FullBiliClient)
	if !ok {
		return errors.New("runner requires a full bili client")
	}
	accounts, err := r.Store.List(ctx)
	if err != nil {
		return err
	}
	var runErr error
	for i, account := range accounts {
		r.logf("######### account %d uid=%s #########", i, account.UID)
		if err := r.runAccount(ctx, full, account); err != nil {
			r.logf("account %s failed: %v", account.UID, err)
			runErr = err
		}
	}
	return runErr
}

func (r *Runner) runAccount(ctx context.Context, client FullBiliClient, account store.Account) error {
	ck, err := cookie.Parse(account.Cookie)
	if err != nil {
		return err
	}
	if ck.Buvid3() == "" {
		if err := client.SetCookie(ctx, ck); err != nil {
			r.logf("set cookie failed: %v", err)
		} else {
			account.Cookie = ck.String()
			_ = r.Store.Save(ctx, account)
		}
	}
	user, err := client.Nav(ctx, ck)
	if err != nil {
		return err
	}
	r.logf("login ok: %s uid=%s", user.Uname, ck.UserID())

	reward, err := client.DailyReward(ctx, ck)
	if err != nil {
		return err
	}
	signer := bili.SignerFromNav(user)
	if (!reward.Watch && r.Config.Task.WatchVideo) || (!reward.Share && r.Config.Task.ShareVideo) {
		video, err := r.pickVideo(ctx, ck, signer)
		if err != nil {
			return err
		}
		if !reward.Watch && r.Config.Task.WatchVideo {
			if err := client.WatchVideo(ctx, ck, video); err != nil {
				r.logf("watch video failed: %v", err)
			}
		}
		if !reward.Share && r.Config.Task.ShareVideo {
			if err := client.ShareVideo(ctx, ck, video); err != nil {
				r.logf("share video failed: %v", err)
			}
		}
	}
	if !shouldDonate(r.Config.Task.SaveCoinsWhenLv6, user.LevelInfo.CurrentLevel) {
		r.logf("skip coin task because account is lv%d", user.LevelInfo.CurrentLevel)
		return nil
	}
	return r.donateCoins(ctx, client, ck, signer)
}

func (r *Runner) donateCoins(ctx context.Context, client FullBiliClient, ck *cookie.Cookie, signer wbi.Signer) error {
	exp, err := client.DonateCoinExp(ctx, ck)
	if err != nil {
		return err
	}
	balance, err := client.CoinBalance(ctx, ck)
	if err != nil {
		return err
	}
	need := neededCoins(r.Config.Task.NumberOfCoins, exp, balance, r.Config.Task.ProtectedCoins)
	if need <= 0 {
		r.logf("coin task already complete or balance protected")
		return nil
	}
	success := 0
	for attempts := 0; attempts < 10 && success < need; attempts++ {
		video, err := r.pickVideo(ctx, ck, signer)
		if err != nil {
			continue
		}
		coins, err := client.ArchiveCoins(ctx, ck, video.AID)
		if err != nil || coins.Multiply >= 2 {
			continue
		}
		if err := client.AddCoin(ctx, ck, video, r.Config.Task.SelectLike); err != nil {
			continue
		}
		success++
		time.Sleep(r.Config.Task.RequestInterval)
	}
	if success < need {
		return fmt.Errorf("coin task incomplete: success %d need %d", success, need)
	}
	return nil
}

func (r *Runner) pickVideo(ctx context.Context, ck *cookie.Cookie, signer wbi.Signer) (bili.Video, error) {
	for _, upID := range shuffled(r.Config.Task.SupportUpIDs) {
		data, err := r.Bili.SearchUPVideos(ctx, ck, signer, upID, 1)
		if err != nil || data.Page.Count <= 0 {
			continue
		}
		page := 1
		if data.Page.Count > 1 {
			page = rand.Intn(data.Page.Count) + 1
			data, err = r.Bili.SearchUPVideos(ctx, ck, signer, upID, page)
			if err != nil {
				continue
			}
		}
		if len(data.List.VList) > 0 {
			return data.List.VList[0], nil
		}
	}

	videos, err := r.Bili.Ranking(ctx)
	if err != nil {
		return bili.Video{}, err
	}
	if len(videos) == 0 {
		return bili.Video{}, errors.New("ranking returned no videos")
	}
	return videos[rand.Intn(len(videos))], nil
}

func neededCoins(targetCoins int, donatedExp int, balance float64, protectedCoins int) int {
	if targetCoins <= 0 {
		return 0
	}
	already := donatedExp / 10
	need := targetCoins - already
	if need <= 0 {
		return 0
	}
	available := int(math.Floor(balance)) - protectedCoins
	if available <= 0 {
		return 0
	}
	if need > available {
		return available
	}
	return need
}

func shouldDonate(saveCoinsWhenLv6 bool, currentLevel int) bool {
	return !(saveCoinsWhenLv6 && currentLevel >= 6)
}

func shuffled(ids []int64) []int64 {
	out := append([]int64(nil), ids...)
	rand.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func (r *Runner) logf(format string, args ...any) {
	if r.Logger != nil {
		r.Logger.Printf(format, args...)
	}
}
