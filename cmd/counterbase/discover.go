package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"

	"github.com/danp/counterbase/source"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type discoverExec struct {
}

func newDiscoverCmd() *ffcli.Command {
	var (
		fs = flag.NewFlagSet("counterbase discover", flag.ExitOnError)
	)

	ce := &discoverExec{}

	return &ffcli.Command{
		Name:       "discover",
		ShortUsage: "counterbase discover",
		ShortHelp:  "discover counters",
		FlagSet:    fs,
		Exec:       ce.exec,
	}
}

func (d discoverExec) exec(ctx context.Context, args []string) error {
	var ht source.HalifaxTransit

	counters, err := ht.Counters(ctx)
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(counters)
}
