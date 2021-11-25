package main

import (
	"github.com/google/uuid"
	cpUtil "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type archiveTestData struct {
	archiveType string
	tmp         string
}

func TestService(t *testing.T) {
	for _, test := range []archiveTestData{
		{
			archiveType: "rar",
			tmp:         uuid.New().String(),
		}, {
			archiveType: "zip",
		},
	} {
		ArchiveTest(t, test)
	}
}

func ArchiveTest(t *testing.T, test archiveTestData) {
	path, err := ioutil.TempDir("", "unziploc")
	assert.NoError(t, err)
	defer func() {
		os.RemoveAll(path)
	}()
	c := Config{
		p:                   path,
		paths:               []string{path},
		writeDelayDuration:  time.Microsecond,
		tickerTimerDuration: time.Microsecond,
		pathExpireDuration:  time.Second * 10,
	}
	if test.tmp != "" {
		os.MkdirAll(test.tmp, os.ModeDir)
		defer func() {
			os.RemoveAll(test.tmp)
		}()
		c.tmpDir = test.tmp
	}
	s := New(&c)
	go s.Start()
	time.Sleep(time.Second)
	tmpDataDir := filepath.Join(path, test.archiveType)
	os.MkdirAll(tmpDataDir, os.ModeDir)
	cpUtil.Copy(filepath.Join("testdata", test.archiveType), tmpDataDir)
	time.Sleep(time.Second)
	assertDataExists(t, path, s)
	s.Stop()
}

func assertDataExists(t *testing.T, path string, s *Service) {
	dataFound := false
	filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if strings.HasSuffix(path, "somestuff.txt") {
			dataFound = true
			return nil
		}
		return nil
	})
	assert.Truef(t, dataFound, "Data not found")
}

func TestExpire(t *testing.T) {
	path, err := ioutil.TempDir("", "unziploc")
	assert.NoError(t, err)
	defer func() {
		os.RemoveAll(path)
	}()
	s := New(&Config{
		p:                   path,
		paths:               []string{path},
		writeDelayDuration:  time.Millisecond * 100,
		tickerTimerDuration: time.Millisecond * 100,
		pathExpireDuration:  time.Millisecond * 500,
	})
	go s.Start()
	time.Sleep(time.Second)
	tmpDataDir := filepath.Join(path, "expire")
	os.MkdirAll(tmpDataDir, os.ModeDir)
	f, err := ioutil.TempFile(tmpDataDir, "thing.zip")
	assert.NoError(t, err)
	f.Write([]byte("Hello"))
	f.Close()
	tracking, ok := s.Data[tmpDataDir]
	assert.NotNil(t, tracking)
	assert.True(t, ok)
	time.Sleep(time.Second)
	dne, ok := s.Data[tmpDataDir]
	assert.Nil(t, dne)
	assert.False(t, ok)
	s.Stop()
}

func TestIsPathExists(t *testing.T) {
	t.Parallel()
	dne, err := IsPathExists("/do/not/exist")
	assert.False(t, dne)
	assert.NoError(t, err)
	exists, err := IsPathExists("testdata")
	assert.True(t, exists)
	assert.NoError(t, err)
}
