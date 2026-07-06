package login

import (
	"context"
	"errors"
	"time"

	"bili-up/internal/cookie"
	"bili-up/internal/store"
)

type QRCode struct {
	URL string
	Key string
}

type BiliClient interface {
	GenerateQRCode(context.Context) (QRCode, error)
	PollQRCode(context.Context, string) (*cookie.Cookie, bool, error)
	SetCookie(context.Context, *cookie.Cookie) error
}

type Service struct {
	Bili      BiliClient
	Store     store.Store
	PollLimit int
	PollDelay time.Duration
}

func (s Service) Login(ctx context.Context, showQRCode func(string)) (store.Account, error) {
	if s.PollLimit <= 0 {
		s.PollLimit = 24
	}
	if s.PollDelay <= 0 {
		s.PollDelay = 5 * time.Second
	}
	qr, err := s.Bili.GenerateQRCode(ctx)
	if err != nil {
		return store.Account{}, err
	}
	if showQRCode != nil {
		showQRCode(qr.URL)
	}

	var ck *cookie.Cookie
	for i := 0; i < s.PollLimit; i++ {
		got, done, err := s.Bili.PollQRCode(ctx, qr.Key)
		if err != nil {
			return store.Account{}, err
		}
		if done {
			ck = got
			break
		}
		select {
		case <-ctx.Done():
			return store.Account{}, ctx.Err()
		case <-time.After(s.PollDelay):
		}
	}
	if ck == nil {
		return store.Account{}, errors.New("login timeout")
	}
	if err := s.Bili.SetCookie(ctx, ck); err != nil {
		return store.Account{}, err
	}
	account := store.Account{
		UID:    ck.UserID(),
		Cookie: ck.String(),
	}
	if err := s.Store.Save(ctx, account); err != nil {
		return store.Account{}, err
	}
	return account, nil
}
