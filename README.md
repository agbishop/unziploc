# unziploc
Watches for zips/tars/rars and unzips them

Options:
```bash
PATHS = /data #Comma seperated list of paths to watch
WRITE_DELAY = 1m #How long to wait after the last write to check for a zip ( for copies to finish )
PATH_EXPIRE_DURATION = 1h # How long to wait before removing the new path. This is for errors or stale data
TIMER_TICKER = 10s # How long to timer loops ticks.
DEBUG = true # Debug logging
```

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
