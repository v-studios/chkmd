// Package main provides a program to check for AVAIL required metadata
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v1"
)

const (
	exifDate     = "2006:01:02 15:04:05"
	exifNanoDate = "2006:01:02 15:04:05.00"
)

var (
	cfgfile = flag.String("c", "dev-config.yaml", "The config file to read from.")

	mimeTypes = make(map[string]bool)
	zeroTime  = time.Time{}
)

type config struct {
	MimeTypes []string `yaml:"mime_types"`
}

type exif struct {
	Data map[string]string
}

func newExif() exif {
	return exif{Data: map[string]string{}}
}

func (e exif) DateCreated() (time.Time, error) {
	d := e.Data["Date/Time Original"]
	return parseDate(d)
}

func (e exif) HasDateCreated() bool {
	d, err := e.DateCreated()
	if err != nil {
		return false
	}
	if d == zeroTime {
		return false
	}
	return true
}

func (e exif) Keywords() string {
	return e.Data["Keywords"]
}

func (e exif) HasKeywords() bool {
	return e.Keywords() != ""
}

func (e exif) Description() string {
	return e.Data["Description"]
}

func (e exif) HasDescription() bool {
	return e.Description() != ""
}

func parseDate(d string) (time.Time, error) {
	var (
		t      time.Time
		err    error
		format string
	)
	d = strings.TrimSpace(d)
	if strings.Contains(d, ".") {
		format = exifNanoDate
	} else {
		format = exifDate
	}
	t, err = time.Parse(format, d)
	if err != nil {
		return t, err
	}
	return t, err

}

func getExifData(p string) (exif, error) {
	exif := newExif()

	cmd := exec.Command("exiftool", p)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return exif, err
	}

	cmdOut := strings.Trim(out.String(), " \r\n")
	lines := strings.Split(cmdOut, "\n")

	for _, line := range lines {
		exif.Data[strings.TrimSpace(line[0:32])] = strings.TrimSpace(line[33:])
	}

	return exif, nil
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
