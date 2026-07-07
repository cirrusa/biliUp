package tasks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"bili-up/internal/bili"
	"bili-up/internal/config"
	"bili-up/internal/cookie"
	"bili-up/internal/store"
	"bili-up/internal/wbi"
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
		return errors.New("任务执行器需要完整的哔哩哔哩客户端实现")
	}
	accounts, err := r.Store.List(ctx)
	if err != nil {
		return err
	}
	var runErr error
	for i, account := range accounts {
		r.logf("######### 开始处理账号 %d，uid=%s #########", i, account.UID)
		if err := r.runAccount(ctx, full, account); err != nil {
			r.logf("账号 %s 执行失败: %v", account.UID, err)
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
			r.logf("补全 Cookie 失败: %v", err)
		} else {
			account.Cookie = ck.String()
			_ = r.Store.Save(ctx, account)
		}
	}
	user, err := client.Nav(ctx, ck)
	if err != nil {
		return err
	}
	r.logf("登录校验成功: 用户名=%s uid=%s", user.Uname, ck.UserID())

	reward, err := client.DailyReward(ctx, ck)
	if err != nil {
		return err
	}
	signer := bili.SignerFromNav(user)
	r.logTaskStatus("watch", reward.Watch, r.Config.Task.WatchVideo)
	r.logTaskStatus("share", reward.Share, r.Config.Task.ShareVideo)
	if (!reward.Watch && r.Config.Task.WatchVideo) || (!reward.Share && r.Config.Task.ShareVideo) {
		video, err := r.pickVideo(ctx, ck, signer)
		if err != nil {
			return err
		}
		if !reward.Watch && r.Config.Task.WatchVideo {
			r.logf("观看任务开始: aid=%d bvid=%s 标题=%q", video.AID, video.BVID, video.Title)
			if err := client.WatchVideo(ctx, ck, video); err != nil {
				r.logf("观看视频失败: %v", err)
			} else {
				r.logf("观看任务完成: aid=%d bvid=%s", video.AID, video.BVID)
			}
		}
		if !reward.Share && r.Config.Task.ShareVideo {
			r.logf("分享任务开始: aid=%d bvid=%s 标题=%q", video.AID, video.BVID, video.Title)
			if err := client.ShareVideo(ctx, ck, video); err != nil {
				r.logf("分享视频失败: %v", err)
			} else {
				r.logf("分享任务完成: aid=%d bvid=%s", video.AID, video.BVID)
			}
		}
	}
	if !shouldDonate(r.Config.Task.SaveCoinsWhenLv6, user.LevelInfo.CurrentLevel) {
		r.logf("跳过投币任务: 当前账号等级为 lv%d，且已启用满级保币", user.LevelInfo.CurrentLevel)
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
		r.logf("投币任务跳过: 今日投币已完成或当前硬币余额处于保护阈值")
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
		return fmt.Errorf("投币任务未完成: 成功 %d 个，还需要 %d 个", success, need)
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
		return bili.Video{}, errors.New("排行榜接口没有返回可用视频")
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

func (r *Runner) logTaskStatus(task string, done bool, enabled bool) {
	taskName := map[string]string{
		"watch": "观看",
		"share": "分享",
	}[task]
	if taskName == "" {
		taskName = task
	}
	switch {
	case !enabled:
		r.logf("%s任务跳过: 配置已禁用", taskName)
	case done:
		r.logf("%s任务跳过: 今日已完成", taskName)
	default:
		r.logf("%s任务待执行: 本次将尝试执行", taskName)
	}
}
