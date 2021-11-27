package main

import (
	"context"
	"github.com/google/uuid"
	otaiCopy "github.com/otiai10/copy"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gobuffalo/envy"
	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
)

type (
	WorkerData struct {
		Path    string
		Start   time.Time
		Expire  time.Time
		Process bool
	}
	Service struct {
		mux           sync.Mutex
		log           logrus.FieldLogger
		Data          map[string]*WorkerData
		daemonCtx     context.Context
		daemonCtxStop context.CancelFunc
		Config
	}
)

func New(c *Config) *Service {
	s := Service{}
	debug, _ := strconv.ParseBool(envy.Get("DEBUG", "true"))
	logger := logrus.New()
	if debug {
		logger.SetLevel(logrus.DebugLevel)
	}
	s.log = logger
	s.Data = map[string]*WorkerData{}
	if c != nil {
		s.Config = *c
	} else {
		s.Config = s.cli()
	}
	go s.fileProcessorTimerLoop()
	return &s
}

func (s *Service) fileProcessorTimerLoop() {
	ticker := time.NewTimer(s.tickerTimerDuration)
	for {
		z := <-ticker.C
		s.CheckAndUnzip(ticker, z)
	}
}

func IsPathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *Service) Stop() {
	s.daemonCtxStop()
}

func (s *Service) Start() {
	s.log.Infof("Service started. Watching paths %s", s.paths)
	s.daemonCtx, s.daemonCtxStop = context.WithCancel(context.Background())
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
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done(): // if cancel() execute
				<-done
				return
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
	}(s.daemonCtx)
	for i := range s.paths {
		if err := watcher.Add(s.paths[i]); err != nil {
			s.log.Fatal(err)
		}
	}

	<-done
}

func (s *Service) CheckAndUnzip(ticker *time.Timer, z time.Time) {
	s.mux.Lock()
	for path, d := range s.Data {
		if z.After(d.Expire) {
			s.log.Infof("path %s expired", path)
			delete(s.Data, path)
		} else if z.After(d.Start) && !s.Data[path].Process {
			s.Data[path].Process = true
			go s.ProcessNewRarFile(path)
		}
	}
	s.mux.Unlock()
	ticker.Reset(s.tickerTimerDuration)
}

func validSuffix() []string {
	return []string{".rar", ".tar", ".zip"}
}

func (s *Service) unzip(basePath, archivePath string, info fs.FileInfo) (err error) {
	return archiver.Unarchive(archivePath, basePath)
}

func (s *Service) copyWithObfuscation(unzipDir, targetPath string) (err error) {
	return filepath.Walk(unzipDir, func(path string, info fs.FileInfo, err error) error {
		if strings.HasSuffix(path, "extracted") {
			return nil
		}
		if info.IsDir() {
			if err := os.MkdirAll(info.Name(), os.ModeDir); err != nil {
				return err
			}
		} else {
			origPath := filepath.Clean(strings.ReplaceAll(path, unzipDir, targetPath))
			copyPath := path + uuid.New().String()
			copyPath = filepath.Clean(strings.ReplaceAll(copyPath, unzipDir, targetPath))
			if err := otaiCopy.Copy(path, copyPath); err != nil {
				return err
			}
			if err := os.Rename(copyPath, origPath); err != nil {
				return err
			}
		}
		return nil
	})
}

// attempts to move result, if an error occurs when trying to rename,
// then attempt to copy recursively. The copy will copy files as uuids then rename
func (s *Service) moveExtracted(basePath, unzipDir string) (err error) {
	targetPath := filepath.Join(basePath, "extracted")
	if err := os.Rename(unzipDir, targetPath); err != nil {
		s.log.WithError(err).Warnf("failed to link, attempting copy")
		return s.copyWithObfuscation(unzipDir, targetPath)
	}
	return
}

func (s *Service) unzipWithTmpDir(basePath, archivePath string, info fs.FileInfo) (err error) {
	unzipDir := basePath
	tmpDir, err := ioutil.TempDir(s.tmpDir, info.Name())
	if err != nil {
		return err
	}
	unzipDir = filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(unzipDir, os.ModeDir); err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			s.log.WithError(err).Error("clean up err")
		}

	}()
	if err := archiver.Unarchive(archivePath, unzipDir); err != nil {
		return err
	}
	if err := s.moveExtracted(basePath, unzipDir); err != nil {
		return err
	}

	return err
}

func (s *Service) findAndProcessArchive(path string) (err error) {
	suffixes := validSuffix()
	walkErr := filepath.Walk(path, func(archiveFile string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		for i := range suffixes {
			s.log.Debugf("file %s", archiveFile)
			if strings.HasSuffix(archiveFile, suffixes[i]) && !info.IsDir() {
				s.log.Infof("Found archive %s", archiveFile)
				if s.tmpDir != "" {
					if err := s.unzipWithTmpDir(path, archiveFile, info); err != nil {
						return err
					}
				} else {
					if err := s.unzip(path, archiveFile, info); err != nil {
						return err
					}
				}
				return io.EOF // this is to break the loop early
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != io.EOF {
		s.log.Errorf("Walk Err %s", walkErr)
		return walkErr
	}
	return nil
}

func (s *Service) ProcessNewRarFile(path string) (err error) {
	s.log.Debugf("processing %s...", path)
	if err := s.findAndProcessArchive(path); err != nil {
		return err
	}
	s.mux.Lock()
	s.log.Infof("Finished processing %s", path)
	delete(s.Data, path)
	s.mux.Unlock()
	return nil
}

func (s *Service) NewEvent(event fsnotify.Event) {
	s.log.Debugf("fsEvent %s", event.Name)
	s.mux.Lock()
	now := time.Now()
	s.Data[event.Name] = &WorkerData{
		Path:   event.Name,
		Start:  now.Add(s.writeDelayDuration),
		Expire: now.Add(s.pathExpireDuration),
	}
	s.mux.Unlock()
}

func (s *Service) SetPathEventStart(event fsnotify.Event) {
	s.log.Debugf("reset time %s", event.Name)
	s.mux.Lock()
	if data, ok := s.Data[event.Name]; ok {
		s.log.Debugf("setting time for %s", event.Name)
		now := time.Now()
		if !data.Process {
			s.Data[event.Name].Start = now.Add(s.writeDelayDuration)
			s.Data[event.Name].Expire = now.Add(s.pathExpireDuration)
		}
	}
	s.mux.Unlock()
}
