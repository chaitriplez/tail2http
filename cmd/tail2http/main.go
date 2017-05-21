package main

import (
	"flag"
	"net/http"

	"strings"

	"errors"

	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/chaitriplez/tail2http"
	"golang.org/x/time/rate"
)

type Config struct {
	monitorPath   string
	dataPath      string
	filePattern   string
	url           string
	contentType   string
	rateLimit     int
	checkInterval int
	dryRun        bool
}

var stop = false
var config = Config{}
var limiter *rate.Limiter

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	config.monitorPath = *flag.String("monitor-path", "", "directory of file eg. /program/log")
	config.dataPath = *flag.String("data-path", "", "directory for save position file eg. /program/save")
	config.filePattern = *flag.String("file-pattern", ".*", "regexp of file pattern to tail eg. .*log$")
	config.url = *flag.String("url", "", "url of server eg. http://mockbin.org")
	config.contentType = *flag.String("content-type", "application/json", "http header content-type eg. text/html")
	config.rateLimit = *flag.Int("rate-limit", 60, "request per second")
	config.checkInterval = *flag.Int("check-interval", 60, "check file change every x second(s)")
	config.dryRun = *flag.Bool("dry-run", false, "true: not save position file and not request http")
}

func main() {
	flag.Parse()
	log.Infof("Config: %+v", config)

	limiter = rate.NewLimiter(rate.Limit(config.rateLimit), config.rateLimit)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Info("Receive os signal: ", sig)
		stop = true
		done <- stop
	}()

	sleep := make(chan int, 1)
	for !stop {
		process()
		go func() {
			log.Info("Sleep for ", config.checkInterval, " second(s)")
			time.Sleep(time.Second * time.Duration(config.checkInterval))
			sleep <- 1
		}()
		select {
		case <-sleep:
			log.Info("I am awake")
		case <-done:
			log.Info("Sleeping interrupt by os signal")
		}
	}
}

func process() error {
	mFiles, _ := tail2http.ListMonitorFiles(config.monitorPath, config.filePattern)
	dFiles, _ := tail2http.ListDataFiles(config.dataPath)
	send := postTo()

	for fileName, mFile := range mFiles {
		if stop {
			log.Info("Receive stop signal: stop processing next file")
			break
		}
		log.Info("Processing file: ", fileName)

		if dFile, exists := dFiles[fileName]; exists {
			if checkUpdate(dFile, mFile) {
				log.Info("Updating file: ", fileName)

				err := readData(dFile, mFile, send)
				if err != nil {
					return err
				}
			} else {
				log.Info("Old file, skip: ", fileName)
			}
		} else {
			log.Info("New file, start from begin: ", fileName)
			dFile = &tail2http.DataFile{
				Path: config.dataPath,
				Name: fileName,
			}
			err := readData(dFile, mFile, send)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkUpdate(d *tail2http.DataFile, m *tail2http.MonitorFile) bool {
	return m.ModTime > d.ModTime
}

func readData(d *tail2http.DataFile, m *tail2http.MonitorFile, f func(string) error) error {
	p, err := d.Position()
	if err != nil {
		return err
	}
	log.Info("Start position: ", p)

	if err := m.Open(p); err != nil {
		return err
	}
	defer m.Close()

	for !stop && m.Scan() {
		limiter.Wait(context.Background())
		line := m.Text()
		if len(line) > 0 {
			if err := f(line); err != nil {
				savePosition(d, m)
				return err
			}
		}
	}
	savePosition(d, m)

	return nil
}

func savePosition(d *tail2http.DataFile, m *tail2http.MonitorFile) {
	if config.dryRun {
		log.Info("(Dry Run) Save position: ", m.Position())
		return
	}
	log.Info("Save position: ", m.Position())
	if err := d.SaveToFile(m.Position()); err != nil {
		log.Error("Error save position to file[", d.Name, "] position: ", m.Position())
	}
}

func postTo() func(s string) error {
	if config.dryRun {
		return debug
	}
	return httpPost
}

func debug(s string) error {
	log.Info("Receive line: ", s)
	return nil
}

func httpPost(s string) error {
	res, err := http.Post(config.url, config.contentType, strings.NewReader(s))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New(res.Status)
	}
	defer res.Body.Close()
	return nil
}
