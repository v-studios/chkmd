// Package main provides a program to check for AVAIL required metadata
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v1"
)

const (
	exifDate     = "2006:01:02 15:04:05"
	exifNanoDate = "2006:01:02 15:04:05.00"
)

var (
	cfgfile = flag.String("c", "", "The config file to read from.")
	dir     = flag.String("d", "", "The directory to process, recursively.")
	procs   = flag.Int("p", runtime.NumCPU(), "The number of processes to run.")

	mimeTypes = make(map[string]bool)
	wg        sync.WaitGroup
)

// statistics tracks our statistics.
type statistics struct {
	Total    int32
	Relevant int32
	Reject   int32
	Accept   int32
}

// config holds the config.
type config struct {
	MimeTypes []string `yaml:"mime_types"`
}

// Exif is our Exif data structure.
type exif struct {
	Data map[string]string
}

// newExif is an Exif constructor.
func newExif() exif {
	return exif{Data: map[string]string{}}
}

// DateCreated returns a time object representing the creation time.
func (e exif) DateCreated() (time.Time, error) {
	d := e.Data["Date/Time Original"]
	return parseDate(d)
}

// HasDateCreated returns if DateCreated returns a value.
func (e exif) HasDateCreated() bool {
	_, err := e.DateCreated()
	if err != nil {
		return false
	}
	return true
}

// Keywords returns the keywords value.
func (e exif) Keywords() string {
	return e.Data["Keywords"]
}

// HasKeywords returns true if Keywords is non-empty.
func (e exif) HasKeywords() bool {
	return e.Keywords() != ""
}

// Description returns the Description.
func (e exif) Description() string {
	return e.Data["Description"]
}

// Has Description returns true if Description is non-empty.
func (e exif) HasDescription() bool {
	return e.Description() != ""
}

// parseDate, uh, parses the date from the string.
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

// getExifData gets the output of `exiftool p[ath]` and loads it into an exif struct.
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

// readConfig, uh, reads the config, and makes the values available.
func readConfig(p string) {
	var mtypes []string
	switch {
	case p != "":
		b, err := ioutil.ReadFile(p)
		if err != nil {
			log.Fatalf("Couldn't open config file: %s. Error: %s", p, err)
		}
		conf := config{}
		err = yaml.Unmarshal(b, &conf)
		if err != nil {
			log.Fatalf("Error parsing file %s: %s", p, err)
		}
		mtypes = conf.MimeTypes

	case p == "":
		mtypes = defaultTypes
	}
	for _, t := range mtypes {
		mimeTypes[t] = true
	}
}

// makeWalker returns a function suitable for filepath.Walk
func makeWalker(files chan string, stats *statistics, types map[string]bool) func(string, os.FileInfo, error) error {
	return func(p string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		atomic.AddInt32(&stats.Total, 1)
		if types[mime.TypeByExtension(path.Ext(p))] {
			files <- p
			atomic.AddInt32(&stats.Relevant, 1)
			return nil
		}
		return nil
	}
}

func processFiles(files chan string, stats *statistics, wg *sync.WaitGroup) {
	defer wg.Done()
	for p := range files {
		e, err := getExifData(p)
		if err != nil {
			log.Printf("Error processing %s: %s", p, err)
		}
		if e.HasDateCreated() && (e.HasKeywords() || e.HasDescription()) {
			atomic.AddInt32(&stats.Accept, 1)
		} else {
			atomic.AddInt32(&stats.Reject, 1)
		}
	}
}

func main() {
	flag.Parse()
	if *dir == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	readConfig(*cfgfile)
	files := make(chan string, 64)
	stats := &statistics{}

	go func() {
		filepath.Walk(*dir, makeWalker(files, stats, mimeTypes))
		close(files)
	}()

	for i := 0; i < *procs; i++ {
		wg.Add(1)
		go processFiles(files, stats, &wg)
	}

	wg.Wait()
	fmt.Printf("%#v\n", stats)
	fmt.Println("Do stats...")
}
