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
		},
		{
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
		time.Sleep(time.Second)
		assert.NoError(t, os.RemoveAll(path))
	}()
	tmpDir := ""
	if test.tmp != "" {
		testTmpDir, err := ioutil.TempDir("", test.tmp)
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, os.RemoveAll(test.tmp))
		}()
		tmpDir = testTmpDir
	}
	c := Config{
		p:                   path,
		paths:               []string{path},
		writeDelayDuration:  time.Millisecond * 2,
		tickerTimerDuration: time.Millisecond,
		pathExpireDuration:  time.Second * 3,
		tmpDir:              tmpDir,
	}

	s := New(&c)
	go s.Start()
	time.Sleep(time.Millisecond)
	tmpDataDir := filepath.Join(path, test.archiveType)
	assert.NoError(t, os.MkdirAll(tmpDataDir, os.ModeDir))
	assert.NoError(t, cpUtil.Copy(filepath.Join("testdata", test.archiveType)+"/", tmpDataDir))
	time.Sleep(time.Second * 5)
	assertDataExists(t, path)
	s.Stop()
}

func assertDataExists(t *testing.T, path string) {
	dataFound := false
	assert.NoError(t, filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if strings.Contains(path, "somestuff.txt") {
			dataFound = true
			return nil
		}
		return nil
	}))
	assert.Truef(t, dataFound, "Data not found")
}

func TestExpire(t *testing.T) {
	path, err := ioutil.TempDir("", "unziploc")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(path))
	}()
	s := New(&Config{
		p:                   path,
		paths:               []string{path},
		writeDelayDuration:  time.Second,
		tickerTimerDuration: time.Millisecond * 100,
		pathExpireDuration:  time.Second * 3,
	})
	go s.Start()
	time.Sleep(time.Second)
	tmpDataDir := filepath.Join(path, "expire")
	assert.NoError(t, os.MkdirAll(tmpDataDir, os.ModeDir))
	f, err := ioutil.TempFile(tmpDataDir, "thing.zip")
	assert.NoError(t, err)
	_, WErr := f.Write([]byte("Hello"))
	assert.NoError(t, WErr)
	assert.NoError(t, f.Close())
	time.Sleep(time.Second)
	tracking, ok := s.Data[tmpDataDir]
	assert.NotNil(t, tracking)
	assert.True(t, ok)
	time.Sleep(time.Second * 4)
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

func TestCopyWithObfuscation(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "unziploc")
	assert.NoError(t, err)
	path := filepath.Join(tmpDir, "extracted", "stuff", "things")
	assert.NoError(t, os.MkdirAll(path, os.ModeDir))
	f, err := os.Create(filepath.Join(path, "test.file"))
	assert.NoError(t, err)
	_, WErr := f.Write([]byte("Hello"))
	assert.NoError(t, WErr)
	assert.NoError(t, f.Close())
	tmpDir2, err := ioutil.TempDir("", "unziploc")
	assert.NoError(t, err)
	assert.NoError(t, CopyWithObfuscation(tmpDir, tmpDir2))
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, os.RemoveAll(tmpDir2))
	}()
	copyPath := filepath.Join(tmpDir2, "extracted", "stuff", "things", "test.file")
	assert.FileExists(t, copyPath)
}
