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

func TestIPTCDateTime(t *testing.T) {
	PST, _ := time.LoadLocation("America/Los_Angeles")
	EST, _ := time.LoadLocation("America/New_York")
	testValues := []struct {
		date string
		time string
		want time.Time
	}{
		{"2015:01:09", "01:32:16.90	", time.Date(2015, time.January, 9, 01, 32, 16, 900000000, time.UTC)},
		{" 2015:01:09", "01:32:16.90\n ", time.Date(2015, time.January, 9, 01, 32, 16, 900000000, time.UTC)},
		{" 2015:01:09", "01:32:16\n ", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{" 2015:01:09", "01:32:16", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{" 2015:01:09", "", time.Date(2015, time.January, 9, 0, 0, 0, 0, time.UTC)},
		{" 2015:01:09", "01:32:16", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{"2015:01:09", "01:32:16-08:00", time.Date(2015, time.January, 9, 01, 32, 16, 0, PST)},
		{"2015:01:09", "01:32:16.23-05:00", time.Date(2015, time.January, 9, 01, 32, 16, 230000000, EST)},
	}
	for _, tv := range testValues {
		e := exif{
			IPTC: map[string]string{
				"DateCreated": tv.date,
				"TimeCreated": tv.time,
			},
		}
		got, err := e.DateCreated()
		equals(t, got.Sub(tv.want), time.Duration(0))
		equals(t, nil, err)
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
		{" 2015:01:09", time.Date(2015, time.January, 9, 0, 0, 0, 0, time.UTC)},
		{" 2015:01:09 01:32:16", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{"2015:01:09 01:32:16-08:00", time.Date(2015, time.January, 9, 01, 32, 16, 0, PST)},
		{"2015:01:09 01:32:16.23-05:00", time.Date(2015, time.January, 9, 01, 32, 16, 230000000, EST)},
	}
	for _, tv := range testValues {
		e := exif{
			Exif: map[string]string{
				"DateTimeOriginal": tv.key,
			},
		}
		got, err := e.DateCreated()
		equals(t, got.Sub(tv.want), time.Duration(0))
		equals(t, nil, err)
	}
}

func TestXMPDateTime(t *testing.T) {
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
		{" 2015:01:09", time.Date(2015, time.January, 9, 0, 0, 0, 0, time.UTC)},
		{" 2015:01:09 01:32:16", time.Date(2015, time.January, 9, 01, 32, 16, 0, time.UTC)},
		{"2015:01:09 01:32:16-08:00", time.Date(2015, time.January, 9, 01, 32, 16, 0, PST)},
		{"2015:01:09 01:32:16.23-05:00", time.Date(2015, time.January, 9, 01, 32, 16, 230000000, EST)},
	}
	for _, tv := range testValues {
		e := exif{
			XMP: map[string]string{
				"DateCreated": tv.key,
			},
		}
		got, err := e.DateCreated()
		equals(t, got.Sub(tv.want), time.Duration(0))
		equals(t, nil, err)
	}
}

func TestHasDateCreated(t *testing.T) {
	testValues := []struct {
		key  string
		want bool
	}{
		{"2015:01:09 01:32:16.90", true},
		{"", false},
	}
	for _, tv := range testValues {
		e := exif{Exif: map[string]string{"DateTimeOriginal": tv.key}}
		equals(t, e.HasDateCreated(), tv.want)

		e = exif{IPTC: map[string]string{"DateCreated": tv.key}}
		equals(t, e.HasDateCreated(), tv.want)

		e = exif{XMP: map[string]string{"DateCreated": tv.key}}
		equals(t, e.HasDateCreated(), tv.want)
	}
}

func TestKeyWords(t *testing.T) {
	values := []struct {
		key string
	}{
		{"Some, Key, Words, With space"},
		{"a"},
	}

	for _, v := range values {
		e := newExif()
		e.IPTC["Keywords"] = v.key
		equals(t, e.Keywords(), v.key)

		e = newExif()
		e.XMP["Subject"] = v.key
		equals(t, e.Keywords(), v.key)
	}
}

func TestHasKeywords(t *testing.T) {
	values := []struct {
		key  string
		want bool
	}{
		{"Some, Key, Words, With space", true},
		{"", false},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC["Keywords"] = v.key
		equals(t, e.HasKeywords(), v.want)

		e = newExif()
		e.XMP["Subject"] = v.key
		equals(t, e.HasKeywords(), v.want)
	}
}

func TestDescription(t *testing.T) {
	values := []struct {
		value string
	}{
		{"This is a descripton."},
		{"Another fun description"},
		{"Great non descripiton"},
		{"汉字/漢字"},
		{""},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC["Caption-Abstract"] = v.value
		equals(t, e.Description(), v.value)

		e = newExif()
		e.Exif["ImageDescription"] = v.value
		equals(t, e.Description(), v.value)

		e = newExif()
		e.XMP["Description"] = v.value
		equals(t, e.Description(), v.value)

	}
}

func TestHasDescription(t *testing.T) {
	values := []struct {
		value string
		want  bool
	}{
		{"This is a descripton.", true},
		{"Another fun description", true},
		{"Great non descripiton", true},
		{"汉字/漢字", true},
		{"", false},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC["Caption-Abstract"] = v.value
		equals(t, e.HasDescription(), v.want)

		e = newExif()
		e.Exif["ImageDescription"] = v.value
		equals(t, e.HasDescription(), v.want)

		e = newExif()
		e.XMP["Description"] = v.value
		equals(t, e.HasDescription(), v.want)

	}
}

func TestIPTCNasaID(t *testing.T) {
	values := []struct {
		jid, jidStr, otr, otrStr, want string
	}{
		{"JobID", "anid", "", "", "anid"},
		{"", "", "OriginalTransmissionReference", "anid", "anid"},
		{"JobID", "jobid", "OriginalTransmissionReference", "otrid", "otrid"},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC[v.jid] = v.jidStr
		e.IPTC[v.otr] = v.otrStr
		equals(t, e.NasaID(), v.want)
	}
}

func TestExifNasaID(t *testing.T) {
	values := []struct {
		key   string
		value string
		want  bool
	}{
		{"ImageUniqueID", "anid", true},
		{"ImageUniqueID", "anotherid", true},
		{"NotAKey", "thisid", false},
		{"Tragedy Key", "badid", false},
	}
	for _, v := range values {
		e := newExif()
		e.Exif[v.key] = v.value
		equals(t, e.HasNasaID(), v.want)
	}
}

func TestXMPOrFileNameNasaID(t *testing.T) {
	values := []struct {
		xid, xidStr, xid2, xid2Str, fn, fnStr, want string
	}{
		{"Identifier", "anid", "TransmissionReference", "otherid", "FileName", "aname", "anid"},
		{"Nope", "anid", "TransmissionReference", "otherid", "FileName", "aname", "otherid"},
		{"Wrong", "anid", "NotThis", "otherid", "FileName", "aname", "aname"},
	}
	for _, v := range values {
		e := newExif()
		e.XMP[v.xid] = v.xidStr
		e.XMP[v.xid2] = v.xid2Str
		e.Data[v.fn] = v.fnStr
		equals(t, e.NasaID(), v.want)
	}
}

func TestHasTitle(t *testing.T) {
	values := []struct {
		on, onStr, h, hStr, t, tStr, want string
	}{
		{"ObjectName", "atitle", "Headline", "othertitle", "Title", "xmpTitle", "atitle"},
		{"Foo", "atitle", "Headline", "othertitle", "Title", "xmpTitle", "othertitle"},
		{"Foo", "atitle", "Bar", "othertitle", "Title", "xmpTitle", "xmpTitle"},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC[v.on] = v.onStr
		e.IPTC[v.h] = v.hStr
		e.XMP[v.t] = v.tStr
		equals(t, e.Title(), v.want)
		equals(t, e.HasTitle(), true)
		e = newExif()
		equals(t, e.HasTitle(), false)
	}
}

func TestLocation(t *testing.T) {
	values := []struct {
		iCity, iCityStr, xCity, xCityStr,
		ipState, ipStateStr, xState, xStateStr,
		iCountry, iCountryStr, xCountry, xCountryStr,
		want string
	}{
		{"City", "Montreal", "City", "Montreal",
			"Province-State", "Quebec", "State", "Quebec",
			"Country-PrimaryLocationName", "Canada", "Country", "Canada",
			"Montreal, Quebec, Canada"},
		{"Nope", "Montreal", "City", "Montreal",
			"Uhuh", "Quebec", "State", "Quebec",
			"No way", "Canada", "Country", "Canada",
			"Montreal, Quebec, Canada"},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC[v.iCity] = v.iCityStr
		e.XMP[v.xCity] = v.xCityStr

		e.IPTC[v.ipState] = v.ipStateStr
		e.XMP[v.xState] = v.xStateStr

		e.IPTC[v.iCountry] = v.iCountryStr
		e.XMP[v.xCountry] = v.xCountryStr

		equals(t, e.Location(), v.want)
		equals(t, e.HasLocation(), true)

		e = newExif()
		equals(t, e.HasLocation(), false)
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
		{"image.jpg", make(chan []string, 1), "apath", "astatus", "areason", []string{"apath", "astatus", "areason", "image", "", "N/A", "Row of power lines receding into mountain range at sunset during rain storm..Kingston, Arizona", "2003-09-01T18:28:44Z", "", "Kingman, Arizona, AZ, balance, color, colour, communicate, communication, communication industry, communications, desert, deserts, electric, electric lines, electrical, electrical energy, electricity, energy, evening, foothill, foothills, horizontal, industries, industry, journey, landscape, landscapes, lighting, line, lines, location, locations, mountain, mountains, network, networked, networking, networks, outdoor, outdoors, outside, physics, power, power line, power lines, power-line, power-lines, powerline, powerlines, progress, progressing, progression, rain, rain shower, rainfall, raining, rainy, row, row of, rows, rural, rural outdoors, series, speed, stack, stacked up, stacks, stretching, sunset, sunsets, sunsets over land, team work, team-work, teamwork, technological, technologies, technology, telephone lines, telephone systems, United States Of America, weather", "image", "JPEG", "N/A", "N/A", "Mark Harmel", "N/A"}},
		{"nomd.jpg", make(chan []string, 1), "apath", "astatus", "areason", []string{"apath", "astatus", "areason", "nomd", "", "N/A", "", "", "", "", "image", "JPEG", "N/A", "N/A", "", "N/A"}},
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
		fmt, fmtStr, m, mStr, want string
		has                        bool
	}{
		{"Format", "image/tiff", "MimeType", "invalid", "image", true},
		{"Nope", "image/tiff", "MIMEType", "video/mpeg", "video", true},
		{"Format", "audio/mpeg", "MIMEType", "image/png", "audio", true},
		{"Format", "application/json", "MIMEType", "text/xml", "", false},
	}
	readConfig("")
	for _, v := range values {
		e := newExif()
		e.XMP[v.fmt] = v.fmtStr
		e.Data[v.m] = v.mStr
		equals(t, e.MediaType(), v.want)
		equals(t, e.HasMediaType(), v.has)
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
		e.Data["MIMEType"] = v.mType
		e.Data["FileType"] = v.fType
		readConfig("")
		equals(t, e.FileFormat(), v.wantString)
		equals(t, e.HasFileFormat(), v.want)
	}
}

func TestPhotographer(t *testing.T) {
	values := []struct {
		ip, ipStr string
		e, eStr   string
		x, xStr   string
		wantStr   string
		want      bool
	}{
		{"By-line", "1 photog",
			"Artist", "2 photog",
			"Artist", "3 photog",
			"1 photog", true},
		{"xxx", "",
			"Artist", "2 photog",
			"Artist", "3 photog",
			"2 photog", true},
		{"xxx", "1 photog",
			"xxx", "2 photog",
			"Artist", "3 photog",
			"3 photog", true},
		{"xxx", "1 photog",
			"xxx", "2 photog",
			"xxx", "3 photog",
			"", false},
	}
	for _, v := range values {
		e := newExif()
		e.IPTC[v.ip] = v.ipStr
		e.Exif[v.e] = v.eStr
		e.XMP[v.x] = v.xStr
		equals(t, e.Photographer(), v.wantStr)
		equals(t, e.HasPhotographer(), v.want)
	}
}

func TestGetExifData(t *testing.T) {
	e, err := getExifData("image.jpg")
	equals(t, err, nil)
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

func TestMain(t *testing.T) {
	want := "Path,Status,Reason,NASA ID,Title,508 Description,Description,Date Created,Location,Keywords,Media Type,File Format,Center,Secondary Creator Credit,Photographer,Album\nnomd.jpg,Incomplete,Minimum metadata not provided,nomd,,N/A,,,,,image,JPEG,N/A,N/A,,N/A\nimage.jpg,Accepted,,image,,N/A,\"Row of power lines receding into mountain range at sunset during rain storm..Kingston, Arizona\",2003-09-01T18:28:44Z,,\"Kingman, Arizona, AZ, balance, color, colour, communicate, communication, communication industry, communications, desert, deserts, electric, electric lines, electrical, electrical energy, electricity, energy, evening, foothill, foothills, horizontal, industries, industry, journey, landscape, landscapes, lighting, line, lines, location, locations, mountain, mountains, network, networked, networking, networks, outdoor, outdoors, outside, physics, power, power line, power lines, power-line, power-lines, powerline, powerlines, progress, progressing, progression, rain, rain shower, rainfall, raining, rainy, row, row of, rows, rural, rural outdoors, series, speed, stack, stacked up, stacks, stretching, sunset, sunsets, sunsets over land, team work, team-work, teamwork, technological, technologies, technology, telephone lines, telephone systems, United States Of America, weather\",image,JPEG,N/A,N/A,Mark Harmel,N/A\n"
	alternative := "Path,Status,Reason,NASA ID,Title,508 Description,Description,Date Created,Location,Keywords,Media Type,File Format,Center,Secondary Creator Credit,Photographer,Album\nimage.jpg,Accepted,,image,,N/A,\"Row of power lines receding into mountain range at sunset during rain storm..Kingston, Arizona\",2003-09-01T18:28:44Z,,\"Kingman, Arizona, AZ, balance, color, colour, communicate, communication, communication industry, communications, desert, deserts, electric, electric lines, electrical, electrical energy, electricity, energy, evening, foothill, foothills, horizontal, industries, industry, journey, landscape, landscapes, lighting, line, lines, location, locations, mountain, mountains, network, networked, networking, networks, outdoor, outdoors, outside, physics, power, power line, power lines, power-line, power-lines, powerline, powerlines, progress, progressing, progression, rain, rain shower, rainfall, raining, rainy, row, row of, rows, rural, rural outdoors, series, speed, stack, stacked up, stacks, stretching, sunset, sunsets, sunsets over land, team work, team-work, teamwork, technological, technologies, technology, telephone lines, telephone systems, United States Of America, weather\",image,JPEG,N/A,N/A,Mark Harmel,N/A\nnomd.jpg,Incomplete,Minimum metadata not provided,nomd,,N/A,,,,,image,JPEG,N/A,N/A,,N/A\n"

	old := os.Stdout // keep backup of the real stdout
	olderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Error opening pipe: %s", err)
	}
	re, we, err := os.Pipe()
	if err != nil {
		t.Fatalf("Error opening pipe: %s", err)
	}
	os.Stdout = w
	os.Stderr = we

	*dir = "."
	main()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf, bufe bytes.Buffer
		_, err := io.Copy(&buf, r)
		if err != nil {
			t.Fatalf("Error copying stdout to buffer: %s", err)
		}
		outC <- buf.String()
		_, err = io.Copy(&bufe, re)
		if err != nil {
			t.Fatalf("Error copying stderr to buffer: %s", err)
		}
	}()

	// back to normal state
	err = w.Close()
	if err != nil {
		t.Fatalf("Error closing writer: %s", err)
	}
	err = we.Close()
	if err != nil {
		t.Fatalf("Error closing writer: %s", err)
	}
	os.Stdout = old // restoring the real stdout
	os.Stderr = olderr
	out := <-outC

	equals(t, out == want || out == alternative, true)
}

// equals fails the test if got is not equal to want.
func equals(tb testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\tgot: %#v\n\n\twant: %#v\033[39m\n\n", filepath.Base(file), line, got, want)
		tb.FailNow()
	}
}
