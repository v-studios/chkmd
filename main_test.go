package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestReadConfig(t *testing.T) {
	testValues := []struct {
		key  string
		want bool
	}{
		{"audio/amr", true},
		{"audio/flac", true},
		{"image/png", true},
	}
	readConfig("test-config.yaml")
	for _, tv := range testValues {
		got := mimeTypes[tv.key]
		equals(t, got, tv.want)
	}
}

func TestReadDefaultConfig(t *testing.T) {
	testValues := []struct {
		key  string
		want bool
	}{
		{"audio/mpeg", true},
		{"image/png", true},
		{"video/mp4", true},
		{"image/webp", false},
	}
	mimeTypes = map[string]bool{} // reset map
	readConfig("")
	for _, tv := range testValues {
		got := mimeTypes[tv.key]
		equals(t, got, tv.want)
	}
}

func TestExifDateTime(t *testing.T) {
	PST, _ := time.LoadLocation("America/Los_Angeles")
	EST, _ := time.LoadLocation("America/New_York")
	testValues := []struct {
		key  string
		want time.Time
	}{
		{"2015:01:09 01:32:16.90", time.Date(2015, time.January, 9, 01, 32, 16, 900000000, time.UTC)},
		{" 2015:01:09 01:32:16.90\n ", time.Date(2015, time.January, 9, 01, 32, 16, 900000000, time.UTC)},
		{" 2015:01:09 01:32:16\n ", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{" 2015:01:09 01:32:16", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{"2015:01:09 01:32:16-08:00", time.Date(2015, time.January, 9, 01, 32, 16, 0, PST)},
		{"2015:01:09 01:32:16.23-05:00", time.Date(2015, time.January, 9, 01, 32, 16, 230000000, EST)},
	}
	for _, tv := range testValues {
		e := exif{
			Data: map[string]string{
				"Date/Time Original": tv.key,
			},
		}
		got, err := e.DateCreated()
		equals(t, got.Sub(tv.want), time.Duration(0))
		equals(t, nil, err)
	}
}

func TestExifHasDateCreated(t *testing.T) {
	testValues := []struct {
		key  string
		want bool
	}{
		{"2015:01:09 01:32:16.90", true},
		{"", false},
	}
	for _, tv := range testValues {
		e := exif{
			Data: map[string]string{
				"Date/Time Original": tv.key,
			},
		}
		equals(t, e.HasDateCreated(), tv.want)
	}
}

func TestExifHasKeywords(t *testing.T) {
	values := []struct {
		key  string
		want bool
	}{
		{"Some, Key, Words, With space", true},
		{"", false},
	}
	for _, v := range values {
		e := newExif()
		e.Data["Keywords"] = v.key
		equals(t, e.HasKeywords(), v.want)
	}
}

func TestExifHasDescription(t *testing.T) {
	values := []struct {
		key   string
		value string
		want  bool
	}{
		{"Caption-Abstract", "This is a descripton.", true},
		{"Description", "Another fun description", true},
		{"A non description key", "Great non descripiton", false},
		{"Caption-Abstract", "汉字/漢字", true},
	}
	for _, v := range values {
		e := newExif()
		e.Data[v.key] = v.value
		equals(t, e.HasDescription(), v.want)
	}
}

func TestExifHasNasaId(t *testing.T) {
	values := []struct {
		key   string
		value string
		want  bool
	}{
		{"Job Identifier", "anid", true},
		{"Original Transmission Reference", "anotherid", true},
		{"File Name", "thisid", true},
		{"Tragedy Key", "badid", false},
	}
	for _, v := range values {
		e := newExif()
		e.Data[v.key] = v.value
		equals(t, e.HasNasaId(), v.want)
	}
}

func TestExifHasLocation(t *testing.T) {
	values := []struct {
		city, region, country string
		wantString            string
		want                  bool
	}{
		{"Montreal", "Quebec", "Canada", "Montreal, Quebec, Canada", true},
		{"Portland", "Oregon", "", "Portland, Oregon", true},
		{"", "California", "US", "California, US", true},
		{"", "", "", "", false},
		{"NYC", "", "", "NYC", true},
	}
	for _, v := range values {
		e := newExif()
		e.Data["City"] = v.city
		e.Data["Province-State"] = v.region
		e.Data["Country-Primary Location Name"] = v.country
		equals(t, e.Location(), v.wantString)
		equals(t, e.HasLocation(), v.want)
	}
}

func TestHasTitle(t *testing.T) {
	values := []struct {
		key   string
		value string
		want  bool
	}{
		{"Object Name", "atitle", true},
		{"Headline", "btitle", true},
		{"Title", "ctitle", true},
		{"Random", "untitled", false},
	}
	for _, v := range values {
		e := newExif()
		e.Data[v.key] = v.value
		equals(t, e.HasTitle(), v.want)
	}
}

func TestMakeOutput(t *testing.T) {
	var (
		want = "one,two\nthree,four\n"
		buf  bytes.Buffer
		og   sync.WaitGroup
	)
	ch := make(chan []string, 2)
	ch <- []string{"one", "two"}
	ch <- []string{"three", "four"}
	close(ch)

	w := bufio.NewWriter(&buf)
	c := csv.NewWriter(w)

	og.Add(1)
	makeOutput(ch, c, &og)
	og.Wait()
	c.Flush()
	equals(t, buf.String(), want)

}

func TestMakeRow(t *testing.T) {
	values := []struct {
		img    string
		ch     chan []string
		path   string
		status string
		reason string
		want   []string
	}{
		{"image.jpg", make(chan []string, 1), "apath", "astatus", "areason", []string{"apath", "astatus", "areason", "image.jpg", "", "N/A", "Row of power lines receding into mountain range at sunset during rain storm..Kingston, Arizona", "2003-09-01T18:28:44Z", "", "Kingman, Arizona, AZ, balance, color, colour, communicate, communication, communication industry, communications, desert, deserts, electric, electric lines, electrical, electrical energy, electricity, energy, evening, foothill, foothills, horizontal, industries, industry, journey, landscape, landscapes, lighting, line, lines, location, locations, mountain, mountains, network, networked, networking, networks, outdoor, outdoors, outside, physics, power, power line, power lines, power-line, power-lines, powerline, powerlines, progress, progressing, progression, rain, rain shower, rainfall, raining, rainy, row, row of, rows, rural, rural outdoors, series, speed, stack, stacked up, stacks, stretching, sunset, sunsets, sunsets over land, team work, team-work, teamwork, technological, technologies, technology, telephone lines, telephone systems, United States Of America, weather", "image", "JPEG", "N/A", "N/A", "Mark Harmel", "N/A"}},
		{"nomd.jpg", make(chan []string, 1), "apath", "astatus", "areason", []string{"apath", "astatus", "areason", "nomd.jpg", "", "N/A", "", "", "", "", "image", "JPEG", "N/A", "N/A", "", "N/A"}},
	}
	for _, v := range values {
		e, err := getExifData(v.img)
		if err != nil {
			t.Errorf("Error getting exif data for %s: %s", v.img, err)
		}
		e.MakeRow(v.ch, v.path, v.status, v.reason)
		got := <-v.ch
		close(v.ch)
		equals(t, got, v.want)

	}
}

func TestMediaType(t *testing.T) {
	values := []struct {
		value, wantString string
		want              bool
	}{
		{"image/tiff", "image", true},
		{"video/mpeg", "video", true},
		{"audio/mpeg", "audio", true},
		{"application/json", "", false},
	}
	readConfig("")
	for _, v := range values {
		e := newExif()
		e.Data["MIME Type"] = v.value
		equals(t, e.MediaType(), v.wantString)
		equals(t, e.HasMediaType(), v.want)
	}
}

func TestFileFormat(t *testing.T) {
	values := []struct {
		mType, fType, wantString string
		want                     bool
	}{
		{"image/tiff", "TIFF", "TIFF", true},
		{"image/png", "PNG", "PNG", true},
		{"image/jpeg", "JPG", "JPG", true},
		{"text/plain", "TXT", "", false},
		{"video/mpeg", "MP4", "MP4", true},
		{"audio/mpeg", "MP3", "MP3", true},
		{"not/real", "faker", "", false},
	}
	readConfig("")
	for _, v := range values {
		e := newExif()
		e.Data["MIME Type"] = v.mType
		e.Data["File Type"] = v.fType
		equals(t, e.FileFormat(), v.wantString)
		equals(t, e.HasFileFormat(), v.want)
	}
}

func TestPhotographer(t *testing.T) {
	values := []struct {
		k, v, wantString string
		want             bool
	}{
		{"By-line", "That photog", "That photog", true},
		{"Other", "Hahahahah", "", false},
	}
	for _, v := range values {
		e := newExif()
		e.Data[v.k] = v.v
		equals(t, e.Photographer(), v.wantString)
		equals(t, e.HasPhotographer(), v.want)
	}
}

func TestGetExifData(t *testing.T) {
	e, err := getExifData("image.jpg")
	equals(t, err, nil)
	equals(t, e.HasDateCreated(), true)
	equals(t, e.HasDescription(), true)
	equals(t, e.HasKeywords(), true)
}

func TestGetExifDataNoFile(t *testing.T) {
	e, err := getExifData("noimage.jpg")
	equals(t, e, newExif())
	equals(t, err.Error(), "exit status 1")
}

func TestMakeWalker(t *testing.T) {
	values := []struct {
		key string
	}{
		{"image.jpg"},
		{"../"},
		{"main_test.go"},
	}
	for _, v := range values {
		ch := make(chan string, 10)
		readConfig("test-config.yaml")
		stats := &statistics{}
		f := makeWalker(ch, stats, mimeTypes)
		fi, statErr := os.Stat(v.key)
		err := f(v.key, fi, statErr)
		equals(t, err, nil)
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(100 * time.Millisecond)
			timeout <- true
		}()

		select {
		case p := <-ch:
			equals(t, p, v.key)
			close(ch)
		case <-timeout:
			close(timeout)
			close(ch)
		}
	}
}

func TestProcessFiles(t *testing.T) {
	values := []struct {
		key    string
		reject int32
		accept int32
	}{
		{"image.jpg", 0, 1},
		{"nomd.jpg", 1, 0},
		{"test-config.yaml", 1, 0},
	}
	for _, v := range values {
		ch := make(chan string, 2)
		rchan := make(chan []string, 1)
		stats := &statistics{}
		wg := &sync.WaitGroup{}
		wg.Add(1)
		ch <- v.key
		// TODO: work out capturing the log message and test it.
		// var buf bytes.Buffer
		// log.SetOutput(&buf)
		close(ch)
		processFiles(ch, rchan, stats, wg)
		close(rchan)
		// log.SetOutput(os.Stderr)
		equals(t, stats.Accept, v.accept)
		equals(t, stats.Reject, v.reject)
		// fmt.Println("XXX", buf.String())
		// equals(t, strings.Contains(buf.String(), "Error processing test-config.yaml: exit status 1"), true)
	}
}

// equals fails the test if got is not equal to want.
func equals(tb testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\tgot: %#v\n\n\twant: %#v\033[39m\n\n", filepath.Base(file), line, got, want)
		tb.FailNow()
	}
}

func TestMain(t *testing.T) {
	want := "Path,Status,Reason,NASA ID,Title,508 Description,Description,Date Created,Location,Keywords,Media Type,File Format,Center,Secondary Creator Credit,Photographer,Album\nnomd.jpg,Incomplete,Minimum metadata not provided,nomd.jpg,,N/A,,,,,image,JPEG,N/A,N/A,,N/A\nimage.jpg,Accepted,,image.jpg,,N/A,\"Row of power lines receding into mountain range at sunset during rain storm..Kingston, Arizona\",2003-09-01T18:28:44Z,,\"Kingman, Arizona, AZ, balance, color, colour, communicate, communication, communication industry, communications, desert, deserts, electric, electric lines, electrical, electrical energy, electricity, energy, evening, foothill, foothills, horizontal, industries, industry, journey, landscape, landscapes, lighting, line, lines, location, locations, mountain, mountains, network, networked, networking, networks, outdoor, outdoors, outside, physics, power, power line, power lines, power-line, power-lines, powerline, powerlines, progress, progressing, progression, rain, rain shower, rainfall, raining, rainy, row, row of, rows, rural, rural outdoors, series, speed, stack, stacked up, stacks, stretching, sunset, sunsets, sunsets over land, team work, team-work, teamwork, technological, technologies, technology, telephone lines, telephone systems, United States Of America, weather\",image,JPEG,N/A,N/A,Mark Harmel,N/A\n"
	alternative := "Path,Status,Reason,NASA ID,Title,508 Description,Description,Date Created,Location,Keywords,Media Type,File Format,Center,Secondary Creator Credit,Photographer,Album\nimage.jpg,Accepted,,image.jpg,,N/A,\"Row of power lines receding into mountain range at sunset during rain storm..Kingston, Arizona\",2003-09-01T18:28:44Z,,\"Kingman, Arizona, AZ, balance, color, colour, communicate, communication, communication industry, communications, desert, deserts, electric, electric lines, electrical, electrical energy, electricity, energy, evening, foothill, foothills, horizontal, industries, industry, journey, landscape, landscapes, lighting, line, lines, location, locations, mountain, mountains, network, networked, networking, networks, outdoor, outdoors, outside, physics, power, power line, power lines, power-line, power-lines, powerline, powerlines, progress, progressing, progression, rain, rain shower, rainfall, raining, rainy, row, row of, rows, rural, rural outdoors, series, speed, stack, stacked up, stacks, stretching, sunset, sunsets, sunsets over land, team work, team-work, teamwork, technological, technologies, technology, telephone lines, telephone systems, United States Of America, weather\",image,JPEG,N/A,N/A,Mark Harmel,N/A\nnomd.jpg,Incomplete,Minimum metadata not provided,nomd.jpg,,N/A,,,,,image,JPEG,N/A,N/A,,N/A\n"

	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	*dir = "."
	main()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out := <-outC

	equals(t, out == want || out == alternative, true)
}
