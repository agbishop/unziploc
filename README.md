# unziploc
[![run tests](https://github.com/agbishop/unziploc/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/agbishop/unziploc/actions/workflows/test.yml)


Watches for zips/tars/rars and unzips them

Options:
```bash
PATHS = /data #Comma seperated list of paths to watch
WRITE_DELAY = 1m #How long to wait after the last write to check for a zip ( for copies to finish )
PATH_EXPIRE_DURATION = 1h # How long to wait before removing the new path. This is for errors or stale data
TIMER_TICKER = 10s # How long to timer loops ticks.
DEBUG = true # Debug logging
TEMP_DIR=/path/to/tmp/dir
```

NOTES ABOUT TEMP_DIR:

Temp dir is for specifying what temp dir to extract the archive to. Due to limitation in docker, you can't rename across
filesystems. If an error occurs while trying to rename the file, unziploc will attempt to copy all files over.
The copy will rename the file to a uuid before renaming it back. This is to prevent any programs watching the directory
from preemptively moving files around.

Example docker-compose
```yaml
version: "3.9"
services:
  unziploc:
    image: blackbird7181/unziploc:latest
    environment:
      PATHS: /path,/stuff,/things
    volumes:
      - "/share/foo:/path"
      - "/my/other/stuff:/stuff"
      - "/things:/things"
```
