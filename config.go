package main

import (
	"flag"
	"strings"
	"time"

	"github.com/gobuffalo/envy"
)

const (
	pathsENV         = "PATHS"
	writeDelayENV    = "WRITE_DELAY"
	pathExpireDurENV = "PATH_EXPIRE_DURATION"
	timerTickerENV   = "TIMER_TICKER"
	tmpDirENV        = "TMP_DIR"
)

type (
	Config struct {
		tmpDir              string
		p                   string
		paths               []string
		writeDelayDuration  time.Duration
		tickerTimerDuration time.Duration
		pathExpireDuration  time.Duration
	}
)

func (s *Service) cli() Config {
	c := Config{}
	flag.StringVar(&c.p, "paths", envy.Get(pathsENV, ""), "comma seperated list of directories to watch")
	flag.StringVar(&c.tmpDir, "tmp-dir", envy.Get(tmpDirENV, ""), "tmp dir to use. must be the same disk")

	writeDelay, err := time.ParseDuration(envy.Get(writeDelayENV, "1m"))
	if err != nil {
		s.log.WithError(err).Fatal("failed to parse write delay")
	}
	flag.DurationVar(&c.writeDelayDuration, "write-delay", writeDelay, "delay to wait after the last write is detected. defaults to 1m")

	tickerDelay, err := time.ParseDuration(envy.Get(timerTickerENV, "10s"))
	if err != nil {
		s.log.WithError(err).Fatal("failed to parse ticker timer")
	}
	flag.DurationVar(&c.tickerTimerDuration, "timer-ticker", tickerDelay, "how fast to run the ticker timer. defaults to 10s")

	pathExpire, err := time.ParseDuration(envy.Get(pathExpireDurENV, "1h"))
	if err != nil {
		s.log.WithError(err).Fatal("failed to parse max wait")
	}
	flag.DurationVar(&c.pathExpireDuration, "max-path-wait", pathExpire, "how long to wait before removing the path. defaults to 1h")

	flag.Parse()
	if c.p == "" {
		s.log.Fatalf("path required")
	}
	c.paths = strings.Split(strings.TrimSpace(c.p), ",")
	for i := range c.paths {
		c.paths[i] = strings.TrimSpace(c.paths[i])
		if exists, _ := IsPathExists(c.paths[i]); !exists {
			s.log.Fatalf("Dir %s doesn't exist", c.paths[i])
		}
	}
	return c
}
