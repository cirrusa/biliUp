package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/robfig/cron/v3"

	"bili-up/internal/bili"
	"bili-up/internal/config"
	"bili-up/internal/cookie"
	"bili-up/internal/login"
	"bili-up/internal/store"
	"bili-up/internal/tasks"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("bili-up", flag.ContinueOnError)
	configPath := fs.String("config", "/app/config/config.json", "path to config json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		printUsage()
		return nil
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	st, err := buildStore(cfg)
	if err != nil {
		return err
	}
	client := bili.NewClient(bili.Options{UserAgent: cfg.Security.UserAgent})
	logger := log.New(os.Stdout, "", log.LstdFlags)

	switch fs.Arg(0) {
	case "login":
		return runLogin(context.Background(), client, st)
	case "run":
		return runDaily(context.Background(), cfg, st, client, logger, fs.Args()[1:])
	case "scheduler":
		return runScheduler(cfg, st, client, logger)
	case "accounts":
		return runAccounts(context.Background(), st)
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}
}

func loadConfig(path string) (config.Config, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return config.Load("")
	}
	return config.Config{}, err
}

func buildStore(cfg config.Config) (store.Store, error) {
	return store.NewJSONStore(cfg.Storage.AccountsFile), nil
}

func runLogin(ctx context.Context, client *bili.Client, st store.Store) error {
	svc := login.Service{
		Bili:      loginAdapter{client: client},
		Store:     st,
		PollLimit: 24,
		PollDelay: 5 * time.Second,
	}
	account, err := svc.Login(ctx, func(url string) {
		printLoginQRCode(url, os.Stdout)
	})
	if err != nil {
		return err
	}
	fmt.Printf("login saved uid=%s\n", account.UID)
	return nil
}

func printLoginQRCode(url string, w io.Writer) {
	fmt.Fprintln(w, "Scan this QR code with Bilibili app:")
	qrterminal.GenerateHalfBlock(url, qrterminal.L, w)
	fmt.Fprintln(w, url)
}

func runDaily(ctx context.Context, cfg config.Config, st store.Store, client *bili.Client, logger *log.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "load accounts without calling bilibili task APIs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dryRun {
		accounts, err := st.List(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("dry-run: loaded %d account(s)\n", len(accounts))
		return nil
	}
	r := tasks.Runner{Bili: client, Store: st, Config: cfg, Logger: logger}
	return r.Run(ctx)
}

func runScheduler(cfg config.Config, st store.Store, client *bili.Client, logger *log.Logger) error {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return err
	}
	c := cron.New(cron.WithLocation(location))
	_, err = c.AddFunc(cfg.Task.Cron, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Task.TimeoutSeconds*20)*time.Second)
		defer cancel()
		if err := runDaily(ctx, cfg, st, client, logger, nil); err != nil {
			logger.Printf("scheduled run failed: %v", err)
		}
	})
	if err != nil {
		return err
	}
	c.Start()
	logger.Printf("scheduler started, cron=%q timezone=Asia/Shanghai", cfg.Task.Cron)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx := c.Stop()
	<-ctx.Done()
	return nil
}

func runAccounts(ctx context.Context, st store.Store) error {
	accounts, err := st.List(ctx)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		fmt.Println("no accounts")
		return nil
	}
	for i, account := range accounts {
		status := "invalid"
		if _, err := cookie.Parse(account.Cookie); err == nil {
			status = "valid"
		}
		fmt.Printf("%d\tuid=%s\tname=%s\tcookie=%s\n", i, account.UID, account.Name, status)
	}
	return nil
}

func printUsage() {
	fmt.Println("Usage: bili-up [--config /path/config.json] <login|run|scheduler|accounts>")
}

type loginAdapter struct {
	client *bili.Client
}

func (a loginAdapter) GenerateQRCode(ctx context.Context) (login.QRCode, error) {
	qr, err := a.client.GenerateQRCode(ctx)
	if err != nil {
		return login.QRCode{}, err
	}
	return login.QRCode{URL: qr.URL, Key: qr.QRCodeKey}, nil
}

func (a loginAdapter) PollQRCode(ctx context.Context, key string) (*cookie.Cookie, bool, error) {
	return a.client.PollQRCode(ctx, key)
}

func (a loginAdapter) SetCookie(ctx context.Context, ck *cookie.Cookie) error {
	return a.client.SetCookie(ctx, ck)
}
