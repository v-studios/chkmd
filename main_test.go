package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
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
