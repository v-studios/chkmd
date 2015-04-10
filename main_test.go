package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
		key  string
		want bool
	}{
		{"This is a descripton.", true},
		{"", false},
		{"汉字/漢字", true},
	}
	for _, v := range values {
		e := newExif()
		e.Data["Description"] = v.key
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

// equals fails the test if got is not equal to want.
func equals(tb testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\tgot: %#v\n\n\twant: %#v\033[39m\n\n", filepath.Base(file), line, got, want)
		tb.FailNow()
	}
}
