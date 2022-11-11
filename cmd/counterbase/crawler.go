package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/danp/counterbase/source"
	"github.com/danp/counterbase/submit"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type crawlerExec struct {
	getDirectory             func(ctx context.Context) (source.Directory, error)
	getSubmitter             func(ctx context.Context) (submit.Submitter, error)
	getQuery                 func(ctx context.Context) (source.Querier, error)
	ecoCounterPrivateDomains *commaSeparatedString
}

func newCrawlerCmd(gd func(ctx context.Context) (source.Directory, error), gs func(ctx context.Context) (submit.Submitter, error), gq func(ctx context.Context) (source.Querier, error)) *ffcli.Command {
	var (
		fs                       = flag.NewFlagSet("counterbase crawler", flag.ExitOnError)
		ecoCounterPrivateDomains commaSeparatedString
	)
	fs.Var(&ecoCounterPrivateDomains, "eco-counter-private-domains", "comma-separated domains to expect for ecocounter://private sources, must have ECO_VISIO_<DOMAIN>_{USERNAME,PASSWORD,USER_ID,DOMAIN_ID} set")

	ce := &crawlerExec{
		getDirectory:             gd,
		getSubmitter:             gs,
		getQuery:                 gq,
		ecoCounterPrivateDomains: &ecoCounterPrivateDomains,
	}

	return &ffcli.Command{
		Name:       "crawler",
		ShortUsage: "counterbase crawler",
		ShortHelp:  "run the source crawler",
		FlagSet:    fs,
		Exec:       ce.exec,
	}
}

func (c crawlerExec) exec(ctx context.Context, args []string) error {
	dir, err := c.getDirectory(ctx)
	if err != nil {
		return err
	}

	sub, err := c.getSubmitter(ctx)
	if err != nil {
		return err
	}

	qu, err := c.getQuery(ctx)
	if err != nil {
		return err
	}

	crawler := &source.Crawler{
		Directory: dir,
		Querier:   qu,
		Submitter: sub,
	}

	var eg source.EcoCounter
	c.addEcoCounterPrivateDomains(&eg)
	crawler.AddGetter("ecocounter", &eg)

	var ht source.HalifaxTransit
	crawler.AddGetter("hfxtransit", &ht)

	return crawler.Run(ctx)
}

func (c crawlerExec) addEcoCounterPrivateDomains(eg *source.EcoCounter) {
	for _, d := range c.ecoCounterPrivateDomains.vals {
		envPrefix := "ECO_VISIO_" + strings.ToUpper(d)
		envUsername := envPrefix + "_USERNAME"
		envPassword := envPrefix + "_PASSWORD"
		envUserID := envPrefix + "_USER_ID"
		envDomainID := envPrefix + "_DOMAIN_ID"

		username, password, userID, domainID := os.Getenv(envUsername), os.Getenv(envPassword), os.Getenv(envUserID), os.Getenv(envDomainID)
		if username == "" || password == "" || domainID == "" {
			log.Printf("eco counter private domain %q missing env %s, %s, %s, or %s, skipping", d, envUsername, envPassword, envUserID, envDomainID)
			continue
		}

		var dom source.EcoCounterPrivateDomain
		dom.Name = d
		dom.Username = username
		dom.Password = password
		dom.UserID = userID
		dom.DomainID = domainID

		if err := eg.AddPrivateDomain(dom); err != nil {
			log.Printf("eco counter private domain %q not added: %s", dom.Name, err)
		}
	}
}
