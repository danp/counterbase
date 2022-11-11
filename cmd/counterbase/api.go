package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danp/counterbase/submit"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type apiExec struct {
	getStorage func(ctx context.Context) (*dbStorage, error)
	addr       *string
}

func newAPICmd(gs func(ctx context.Context) (*dbStorage, error)) *ffcli.Command {
	var (
		fs   = flag.NewFlagSet("counterbase api", flag.ExitOnError)
		addr = fs.String("addr", "127.0.0.1:5000", "listen address for http server")
	)

	ae := &apiExec{
		getStorage: gs,
		addr:       addr,
	}

	return &ffcli.Command{
		Name:       "api",
		ShortUsage: "counterbase api",
		ShortHelp:  "run the submit api",
		FlagSet:    fs,
		Exec:       ae.exec,
	}
}

func (a apiExec) exec(ctx context.Context, args []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	st, err := a.getStorage(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	sh := &submit.Handler{
		Submitter: st,
	}

	mux := http.NewServeMux()
	mux.Handle("/submit", sh)
	mux.HandleFunc("/health", func(http.ResponseWriter, *http.Request) {})

	srv := &http.Server{
		Handler: mux,
		Addr:    *a.addr,
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	return srv.ListenAndServe()
}
