package main

import (
	"flag"
	"github.com/fsnotify/fsnotify"
	"github.com/gobuffalo/envy"
	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type (
	WorkerData struct {
		Path    string
		Start   time.Time
		Process bool
	}
	Service struct {
		mux  sync.Mutex
		log  logrus.FieldLogger
		data map[string]*WorkerData
		Config
	}
	Config struct {
		p                   string
		paths               []string
		writeDelayDuration  time.Duration
		tickerTimerDuration time.Duration
	}
)

func New() *Service {
	s := Service{}
	s.log = logrus.New()
	s.data = make(map[string]*WorkerData)
	ticker := time.NewTimer(s.tickerTimerDuration)
	go func() {
		for {
			select {
			case z := <-ticker.C:
				s.CheckAndUnzip(ticker, z)
			}
		}
	}()
	s.Config = s.init()
	return &s
}

const (
	pathsENV       = "PATHS"
	writeDelayENV  = "WRITE_DELAY"
	timerTickerENV = "TIMER_TICKER"
)

func (s *Service) init() Config {
	c := Config{}
	flag.StringVar(&c.p, "paths", envy.Get(pathsENV, ""), "comma seperated list of directories to watch")

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

	flag.Parse()
	if c.p == "" {
		s.log.Fatalf("path required")
	}
	c.paths = strings.Split(strings.TrimSpace(c.p), ",")
	for i := range c.paths {
		c.paths[i] = strings.TrimSpace(c.paths[i])
		if exists, _ := isExists(c.paths[i]); !exists {
			s.log.Fatalf("Dir %s doesn't exist", c.paths[i])
		}
	}
	return c
}

func isExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *Service) Do() {
	s.log.Infof("Service started. Watching paths %s", s.paths)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Fatal(err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			s.log.WithError(err).Errorf("Error closing watcher")
		}
	}()
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					s.NewEvent(event)
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					s.SetPathEventStart(event)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				s.log.WithError(err).Errorf("fsnotify failure")
			}
		}
	}()
	for i := range s.paths {
		err = watcher.Add(s.paths[i])
	}
	if err != nil {
		s.log.Fatal(err)
	}
	<-done
}

func (s *Service) CheckAndUnzip(ticker *time.Timer, z time.Time) {
	s.mux.Lock()
	for path, d := range s.data {
		if z.After(d.Start) && !s.data[path].Process {
			s.data[path].Process = true
			go s.ProcessNewRarFile(path)
		}
	}
	s.mux.Unlock()
	ticker.Reset(s.tickerTimerDuration)
}

func validSuffix() []string {
	return []string{"rar", "tar"}
}

func (s *Service) ProcessNewRarFile(path string) {
	s.log.Debugf("processing %s...", path)
	suffixes := validSuffix()
	walkErr := filepath.Walk(path, func(p string, info fs.FileInfo, err error) error {
		for i := range suffixes {
			if strings.HasSuffix(p, suffixes[i]) {
				return archiver.Unarchive(p, path)
			}
		}
		return nil
	})
	if walkErr != nil {
		s.log.Errorf("Walk Err %s", walkErr)
	}
	s.mux.Lock()
	delete(s.data, path)
	s.mux.Unlock()
}

func (s *Service) NewEvent(event fsnotify.Event) {
	s.log.Debugf("fsEvent %s", event.Name)
	s.mux.Lock()
	s.data[event.Name] = &WorkerData{
		Path:  event.Name,
		Start: time.Now().Add(s.writeDelayDuration),
	}
	s.mux.Unlock()
}

func (s *Service) SetPathEventStart(event fsnotify.Event) {
	s.log.Debugf("reset time %s", event.Name)
	s.mux.Lock()
	if data, ok := s.data[event.Name]; ok {
		s.log.Debugf("setting time for %s", event.Name)
		if !data.Process {
			s.data[event.Name].Start = time.Now().Add(s.writeDelayDuration)
		}
	}
	s.mux.Unlock()
}
