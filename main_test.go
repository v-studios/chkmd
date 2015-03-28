package main

import (
	"fmt"
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
		got, err := e.DateTime()
		equals(t, got, tv.want)
		equals(t, nil, err)
	}
}

func TestFileWalk(t *testing.T) {

}

// equals fails the test if got is not equal to want.
func equals(tb testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\tgot: %#v\n\n\twant: %#v\033[39m\n\n", filepath.Base(file), line, got, want)
		tb.FailNow()
	}
}
