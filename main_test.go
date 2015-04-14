package main

import (
	"fmt"
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
	testValues := []struct {
		key  string
		want time.Time
	}{
		{"2015:01:09 01:32:16.90", time.Date(2015, time.January, 9, 01, 32, 16, 900000000, time.UTC)},
		{" 2015:01:09 01:32:16.90\n ", time.Date(2015, time.January, 9, 01, 32, 16, 900000000, time.UTC)},
		{" 2015:01:09 01:32:16\n ", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{" 2015:01:09 01:32:16", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
	}
	for _, tv := range testValues {
		e := exif{
			Data: map[string]string{
				"Date/Time Original": tv.key,
			},
		}
		got, err := e.DateCreated()
		equals(t, got, tv.want)
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
