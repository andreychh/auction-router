package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// config is everything the router is told before it starts serving.
type config struct {
	address  string
	partners string
	budget   time.Duration
	level    slog.Level
}

// parseConfig reads args, which must not carry the command name, writing usage
// and complaints to out. Asking for help is reported as [flag.ErrHelp].
func parseConfig(args []string, out io.Writer) (config, error) {
	flags := flag.NewFlagSet("router", flag.ContinueOnError)
	flags.SetOutput(out)

	address := flags.String("addr", ":8080", "address to serve auctions on")
	partners := flags.String("partners", "partners.json", "file naming the demand partners")
	budget := flags.Duration("budget", 200*time.Millisecond,
		"time one whole round of invitations may take")
	level := flags.String("log-level", "info",
		"least severe level to log: debug, info, warn or error")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	if *budget <= 0 {
		return config{}, fmt.Errorf("-budget is %v, want a duration above zero", *budget)
	}

	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(*level)); err != nil {
		return config{}, fmt.Errorf("-log-level %q: %w", *level, err)
	}

	return config{
		address:  *address,
		partners: *partners,
		budget:   *budget,
		level:    parsed,
	}, nil
}
