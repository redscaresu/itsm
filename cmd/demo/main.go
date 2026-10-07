// Command demo plays the scripted browser walkthrough (HLD section 10) in
// two visible Chromium windows.
//
//	demo install   download the Playwright driver and Chromium (once per machine)
//	demo [-bin bin/itsm]
//
// Without DEMO_BASE_URL it starts the itsm binary itself, with the jev key
// from the shell, and stops it at the end. With DEMO_BASE_URL it uses a
// server that is already running.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mxschmitt/playwright-go"

	"github.com/redscaresu/itsm/internal/demo"
	"github.com/redscaresu/itsm/internal/demodata"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
)

// Defaults from HLD 8, slowed so an audience can read each scene.
const (
	defaultPort    = "8080"
	defaultSlowMo  = 1000 * time.Millisecond
	defaultPause   = 5000 * time.Millisecond
	defaultBin     = "bin/itsm"
	installCommand = "install"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Stdout, os.Args, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context, w io.Writer, args []string, getenv func(string) string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(w)
	bin := fs.String("bin", defaultBin, "itsm binary to start when DEMO_BASE_URL is unset")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.Arg(0) == installCommand {
		return playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}, Verbose: true})
	}

	slowMo, err := millis(getenv, "DEMO_SLOWMO_MS", defaultSlowMo)
	if err != nil {
		return err
	}
	pause, err := millis(getenv, "DEMO_PAUSE_MS", defaultPause)
	if err != nil {
		return err
	}
	tickets, err := demodata.Load(demodata.File)
	if err != nil {
		return err
	}

	baseURL := getenv("DEMO_BASE_URL")
	if baseURL == "" {
		port := cmp.Or(getenv("PORT"), defaultPort)
		baseURL = "http://localhost:" + port
		env := []string{"PORT=" + port}
		switch {
		case getenv("DEMO_VIDEO_DIR") != "":
			// A recording replays live jev's recorded answers, so the video
			// is the same every time and needs no key.
			fake := jevfake.New()
			defer fake.Close()
			fake.Script(demodata.Script(tickets))
			env = append(env, "TYPESAFE_BASE_URL="+fake.URL, "TYPESAFE_API_KEY=fake")
		case getenv("TYPESAFE_API_KEY") == "":
			return errors.New("demo: TYPESAFE_API_KEY is not set; run typesafe-key first, or set DEMO_BASE_URL")
		}
		srv, err := demo.StartServer(ctx, *bin, baseURL, env, w)
		if err != nil {
			return fmt.Errorf("%w (run make build first)", err)
		}
		defer func() { _ = srv.Stop() }()
	}

	_, err = demo.Run(ctx, demo.Config{
		BaseURL: baseURL,
		Tickets: tickets,
		SlowMo:  slowMo,
		Pause:   pause,
		// A recording runs headless, so the video is the fixed window size.
		Headless: getenv("DEMO_VIDEO_DIR") != "",
		VideoDir: getenv("DEMO_VIDEO_DIR"),
		Logf:     func(f string, a ...any) { _, _ = fmt.Fprintf(w, f+"\n", a...) },
	})
	return err
}

// millis reads a millisecond count from the environment.
func millis(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("demo: %s must be a whole number of milliseconds, got %q", key, v)
	}
	return time.Duration(n) * time.Millisecond, nil
}
