# tail2http

## Feature

- tail file(s) in directory and send to http server
- support resume last tail position

## Uasge

```sh
# Config
$ tail2http -h
  -check-interval int
        check file change every x second(s) (default 60)
  -content-type string
        http header content-type eg. text/html (default "application/json")
  -data-path string
        directory for save position file eg. /program/save
  -dry-run
        true: not save position file and not request http
  -file-pattern string
        regexp of file pattern to tail eg. .*log$ (default ".*")
  -monitor-path string
        directory of file eg. /program/log
  -rate-limit int
        request per second (default 60)
  -url string
        url of server eg. http://mockbin.org
# Tail log file: ~/log/*log and save current position at ~/log/save/ and sent to url:http://logserver.com/log
$ tail2http -data-path=~/log/save/ -file-pattern=.*log$ -monitor-path=~/log/ -url=http://logserver.com/log
```