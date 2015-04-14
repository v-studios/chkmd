// Package main provides a program to check for AVAIL required metadata
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
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
	na           = "N/A"
)

var (
	cfgfile = flag.String("c", "", "The config file to read from.")
	dir     = flag.String("d", "", "The directory to process, recursively.")
	output  = flag.String("o", "", "A file to output to.")
	procs   = flag.Int("p", runtime.NumCPU(), "The number of processes to run.")
	verbose = flag.Bool("v", false, "Be noisy while processing. Really, just print errors.")

	mimeTypes = make(map[string]bool)
	ingroup   sync.WaitGroup
	outgroup  sync.WaitGroup
	csvHeader = []string{
		"Path",
		"Status",
		"Reason",
		"NASA ID",
		"Title",
		"508 Description",
		"Description",
		"Date Created",
		"Location",
		"Keywords",
		"Media Type",
		"File Format",
		"Center",
		"Secondary Creator Credit",
		"Photographer",
		"Album",
	}
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
	// TODO: Do we need to try IPTC first?
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

// Keywords returns the IPTC keywords value.
func (e exif) Keywords() string {
	return e.Data["Keywords"]
}

// HasKeywords returns true if Keywords is non-empty.
func (e exif) HasKeywords() bool {
	return e.Keywords() != ""
}

// Description returns the Description. Description has been mapped to IPTC Caption-Abstract tag, and also Description in XMP. So we try them in that order.
func (e exif) Description() string {
	var d string
	d = e.Data["Caption-Abstract"]
	if d == "" {
		d = e.Data["Description"]
	}
	return d
}

// Has Description returns true if Description is non-empty.
func (e exif) HasDescription() bool {
	return e.Description() != ""
}

// NasaId tries to return some value for NASA ID. THis is supposed to be the file name, but is also reportedly in ojb Identifier or Original Transmission reference. So we try those and fall back to the File name.
func (e exif) NasaId() string {
	var id string
	id = e.Data["Job Identifier"]
	if id == "" {
		id = e.Data["Original Transmission Reference"]
	}
	if id == "" {
		id = e.Data["File Name"]
	}
	return id
}

// HasNasaId returns if exif.NasaId() returns a non empty string.
func (e exif) HasNasaId() bool {
	return e.NasaId() != ""
}

// Title tries to return a valid title for the asset. This has been mapped to Object Name or Headline in IPTC, but can also be Title in XMP. So we try them in that order.
func (e exif) Title() string {
	var t string
	t = e.Data["Object Name"]
	if t == "" {
		t = e.Data["Headline"]
	}
	if t == "" {
		t = e.Data["Title"]
	}
	return t
}

// HasTitle returns  if exif.Title() returns a non empty string.
func (e exif) HasTitle() bool {
	return e.Title() != ""
}

// Location has been mapped to city and state. this would be mapped to a few IPTC tags: City, Province-State, and Country-Primary Location Name. There are also fields we might be able to use in XMP. It appears that what I see now in XMP are Creator City, Creator ..., But I am not sure that means the subject matter would share the info. So we are only doing IPTC here at this point.
func (e exif) Location() string {
	var city, region, country string
	var addr []string
	city = e.Data["City"]
	if city != "" {
		addr = append(addr, city)
	}
	region = e.Data["Province-State"]
	if region != "" {
		addr = append(addr, region)
	}
	country = e.Data["Country-Primary Location Name"]
	if country != "" {
		addr = append(addr, country)
	}
	return strings.Join(addr, ", ")
}

// HasLocation returns if exif.Location() reurns a non-empty string.
func (e exif) HasLocation() bool {
	return e.Location() != ""
}

// MediaType returns the Media Type by parsing the file's MIME type. It splits
// the MIME type and provides the first part if it is a valid mimeType.
func (e exif) MediaType() string {
	if mimeTypes[e.Data["MIME Type"]] {
		t := strings.Split(e.Data["MIME Type"], "/")[0]
		if t == "audio" || t == "image" || t == "video" {
			return t
		}
	}
	return ""
}

// HasMediaType returns if exif.MediaType() returns a non-empty value.
func (e exif) HasMediaType() bool {
	return e.MediaType() != ""
}

// FileFormat returns the exiftool file format if the MIME type is in mimeTypes.
func (e exif) FileFormat() string {
	if mimeTypes[e.Data["MIME Type"]] {
		return e.Data["File Type"]
	}
	return ""
}

// HasFileFormat returns if exif.FileType() returns a non-empty value.
func (e exif) HasFileFormat() bool {
	return e.FileFormat() != ""
}

// Photographer returns the IPTC By-line. It that fails it falls back to XMP Creator, then Exif Artist.
func (e exif) Photographer() string {
	var p string
	p = e.Data["By-line"]
	if p == "" {
		p = e.Data["Creator"]
	}
	if p == "" {
		p = e.Data["Artist"]
	}
	return p
}

// HasPhotographer returns if exif.Photographer returns a non-empty value.
func (e exif) HasPhotographer() bool {
	return e.Photographer() != ""
}

func (e exif) MakeErrorRow(c chan []string, p string, err error) {
	erow := []string{p, "Rejected", err.Error()}
	for _ = range csvHeader[3:] {
		erow = append(erow, "")
	}
	c <- erow
}

func (e exif) MakeRow(c chan []string, p, status, reason string) {
	var dc string
	if e.HasDateCreated() {
		dto, err := e.DateCreated()
		if err != nil {
			e.MakeErrorRow(c, p, err)
			if *verbose {
				log.Printf("Error getting DateCreated for %s: %s", p, err.Error())
			}
			return
		}
		dc = dto.Format(time.RFC3339)
	}
	row := []string{p,
		status,
		reason,
		e.NasaId(),
		e.Title(),
		na,
		e.Description(),
		dc,
		e.Location(),
		e.Keywords(),
		e.MediaType(),
		e.FileFormat(),
		na,
		na,
		e.Photographer(),
		na,
	}
	c <- row
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

func processFiles(files chan string, results chan []string, stats *statistics, wg *sync.WaitGroup) {
	defer wg.Done()
	var status, reason string
	for p := range files {
		e, err := getExifData(p)
		if err != nil {
			atomic.AddInt32(&stats.Reject, 1)
			e.MakeErrorRow(results, p, err)
			if *verbose {
				log.Printf("Error processing %s: %s\n", p, err)
			}
			return
		}
		if e.HasDateCreated() && (e.HasKeywords() || e.HasDescription()) {
			atomic.AddInt32(&stats.Accept, 1)
			status = "Accepted"
			reason = ""
		} else {
			atomic.AddInt32(&stats.Reject, 1)
			status = "Rejected"
			reason = "Minimum metadata not provided"
		}
		e.MakeRow(results, p, status, reason)
	}
}

func makeOutput(c chan []string, out *csv.Writer, wg *sync.WaitGroup) {
	for result := range c {
		err := out.Write(result)
		if err != nil {
			log.Printf("Error writing row for %s: %s\n", result[0], err.Error())
		}
	}
	wg.Done()
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

	_, err := os.Stat(*dir)
	if err != nil {
		log.Fatalf("Error opening %s: %s\n", *dir, err)
	}
	go func() {
		err := filepath.Walk(*dir, makeWalker(files, stats, mimeTypes))
		if err != nil {
			log.Fatalf("Error opening %s: %s\n", *dir, err)
		}
		close(files)
	}()

	results := make(chan []string, 64)

	for i := 0; i < *procs; i++ {
		ingroup.Add(1)
		go processFiles(files, results, stats, &ingroup)
	}

	var out *csv.Writer
	var f os.File
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			log.Fatalln("Error opening output file: ", err)
		}
		out = csv.NewWriter(f)
	} else {
		out = csv.NewWriter(os.Stdout)
	}
	out.Write(csvHeader)
	out.Flush()

	go func() {
		outgroup.Add(1)
		makeOutput(results, out, &outgroup)
	}()

	ingroup.Wait()
	close(results)
	outgroup.Wait()
	out.Flush()
	f.Close()
	log.Printf("%#v\n", stats)
}
