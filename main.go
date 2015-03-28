// Package main provides a program to check for AVAIL required metadata
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"gopkg.in/yaml.v1"
)

const (
	extDate     = "2006:01:02 15:04:05"
	extNanoDate = "2006:01:02 15:04:05.00"
)

var (
	cfgfile = flag.String("c", "dev-config.yaml", "The config file to read from.")

	mimeTypes = make(map[string]bool)
)

type config struct {
	MimeTypes []string `yaml:"mime_types"`
}

type exif struct {
	Data map[string]string
}

func (e exif) DateTime() (time.Time, error) {
	var (
		t      time.Time
		err    error
		format string
	)
	dt := e.Data["Date/Time Original"]
	dt = strings.TrimSpace(dt)
	if strings.Contains(dt, ".") {
		format = extNanoDate
	} else {
		format = extDate
	}
	t, err = time.Parse(format, dt)
	if err != nil {
		return t, err
	}
	return t, err
}

func getExifData(p string) (exif, error) {
	return exif{}, nil
}

func readConfig(p string) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		log.Fatalf("Couldn't open config file: %s. Error: %s", p, err)
	}
	conf := config{}
	err = yaml.Unmarshal(b, &conf)
	if err != nil {
		log.Fatalf("Error parsing file %s: %s", p, err)
	}
	for _, t := range conf.MimeTypes {
		mimeTypes[t] = true
	}
}
