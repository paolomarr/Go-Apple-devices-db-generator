Apple data wiki parser
===
A tool to fetch Apple devices data from Wikipedia and/or theiphonewiki.com[^1] and
shelve it into a SQLite database

Requirements
---
For Dockerised runs:
- Docker
- Docker compose

For local runs:
- go

Usage
---
### With Docker
```bash
docker-compose up
```

### Without Docker (go required)
```bash
cd go/appledata
go mod tidy
go build
./appledata
```

In either case, the output is a SQLite db file that can be found at `./build/appledata.sqlite` folder.


[^1]: [TheIphoneWiki](https://www.theiphonewiki.com/)'s support has recently been stopped. They seem to have moved to [theapplewiki.com](https://theapplewiki.com/wiki/Main_Page)
