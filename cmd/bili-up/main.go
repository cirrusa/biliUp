package main

import (
	"context"
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
	logger, loggerCloser, err := newRuntimeLogger(0, os.Stdout)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() {
		closeLogger(loggerCloser)
	}()

	var runErr error
	defer func() {
		if runErr != nil {
			logger.Printf("error: %v", runErr)
		}
	}()

	if len(args) == 0 {
		printUsage()
		return nil
	}

	cfg, err := loadConfig()
	if err != nil {
		runErr = err
		return runErr
	}
	nextLogger, nextCloser, err := newRuntimeLogger(cfg.Logging.RetentionDays, os.Stdout)
	if err != nil {
		runErr = fmt.Errorf("init logger: %w", err)
		return runErr
	}
	closeLogger(loggerCloser)
	logger = nextLogger
	loggerCloser = nextCloser
	st, err := buildStore(cfg)
	if err != nil {
		runErr = err
		return runErr
	}
	client := bili.NewClient(bili.Options{UserAgent: cfg.Security.UserAgent})

	command := args[0]
	commandArgs := args[1:]
	switch command {
	case "login":
		runErr = runLogin(context.Background(), client, st)
	case "run":
		runErr = runDaily(context.Background(), cfg, st, client, logger, commandArgs)
	case "scheduler":
		runErr = runScheduler(cfg, st, client, logger)
	case "accounts":
		runErr = runAccounts(context.Background(), st)
	default:
		printUsage()
		runErr = fmt.Errorf("unknown command %q", command)
	}
	return runErr
}

func loadConfig() (config.Config, error) {
	return config.Load()
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
	fmt.Println("Usage: bili-up <login|run|scheduler|accounts>")
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
