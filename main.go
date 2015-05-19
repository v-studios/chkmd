// Package main provides a program to check for AVAIL required metadata
//
// Acquiring metadata can be pretty hairy. An effort is made here to extract
// them in the desired order: IPTC, Exif, XMP. However, the standards have
// changed over time, tools have been known to store data in the wrong fields
// and best of all -- users input data. Users have definitely entered incorrect
// data in fields. One example is to Use the XMP Title for a short description
// or headline, but should be a short alphanumeric identifier analogous to IPTC
// ObjectName or Title.
//
//
// References: (I tried to put these in gdrive, in WP/AVAIL/metadata/resource docs)
//
// IPTC:
//    http://www.photometadata.org/meta-resources-field-guide-to-metadata [IPTC 1]
//    http://www.controlledvocabulary.com/imagedatabases/iptc_core_mapped.pdf  [IPTC 2]
//    http://www.iptc.org/std/photometadata/documentation/CEPIC-IPTC-ImageMetadataHandbook_1.zip (Core_Fields.pdf) [IPTC 3.1]
//    http://www.iptc.org/std/photometadata/documentation/CEPIC-IPTC-ImageMetadataHandbook_1.zip (Extension_Fields.pdf) [IPTC 3.2]
//    http://www.iptc.org/std/photometadata/documentation/CEPIC-IPTC-ImageMetadataHandbook_1.zip (Interactive_Table.pdf) [IPTC 3.3]
//    https://www.iptc.org/std/photometadata/documentation/GenericGuidelines/ [IPTC 4]
//    https://www.iptc.org/std/photometadata/documentation/IPTC-CS5-FileInfo-UserGuide_6.pdf [IPTC 5]
//    https://www.iptc.org/std/IIM/4.2/specification/IIMV4.2.pdf [IPTC 6]
//    https://www.iptc.org/std/IIM/4.1/specification/IPTC-IIM-Schema4XMP-1.0-spec_1.pdf [IPTC 7]
//
// Exif:
//    http://www.exiv2.org/Exif2-2.PDF [Exif 1]
//    http://www.cipa.jp/std/documents/e/DC-008-2010_E.pdf [Exif 2]
//    http://www.cipa.jp/std/documents/e/DC-010-2012_E.pdf [Exif 3]
//
// XMP:
//    http://wwwimages.adobe.com/content/dam/Adobe/en/devnet/xmp/pdfs/XMP%20SDK%20Release%20cc-2014-12/XMPSpecificationPart1.pdf [XMP 1]
//    http://wwwimages.adobe.com/content/dam/Adobe/en/devnet/xmp/pdfs/XMP%20SDK%20Release%20cc-2014-12/XMPSpecificationPart2.pdf [XMP 2]
//    http://www.cipa.jp/std/documents/e/DC-010-2012_E.pdf [Exif 3]
//
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

	yaml "gopkg.in/yaml.v1"
)

const (
	exifDateOnly     = "2006:01:02"
	exifDate         = "2006:01:02 15:04:05"
	exifNanoDate     = "2006:01:02 15:04:05.00"
	exifDateZone     = "2006:01:02 15:04:05-07:00"
	exifNanoDateZone = "2006:01:02 15:04:05.00-07:00"
	na               = "N/A"
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
	Exif map[string]string
	IPTC map[string]string
	XMP  map[string]string
}

// newExif is an Exif constructor.
func newExif() exif {
	return exif{
		Data: map[string]string{},
		Exif: map[string]string{},
		IPTC: map[string]string{},
		XMP:  map[string]string{},
	}
}

////////////////////////////
// Fields in import template
////////////////////////////

// DateCreated returns a time object representing the creation time. We pull
// from IPTC first. IPTC stores date and time separately. So we try getting
// them both and concatenating them. Trimming space will give us just the date.
// Just the time should fail as we don't have a format for them. This failure
// is just as well since a time is pretty pointless without the Year/Month/Day.
// Next we try Exif.DateTimeOriginal and finally XMP.DateCreated.  It appears
// that XMP:CreateDate is when the representation of the resource is created
// and Photoshop:DateCreated is when the copyrightable intellectual property
// was created
//
// This field is available in our import template as 'Date Created'.
func (e exif) DateCreated() (time.Time, error) {
	var d string
	// IPTC 3.1 p.1
	// IPTC 6 pp. 34-35
	// IPTC 7 p. 14                            - photoshop:DateCreated
	// IPTC 7 p. 14                            - photoshop:TimeCreated
	d = strings.TrimSpace(e.IPTC["DateCreated"] + " " + e.IPTC["TimeCreated"])
	if d == "" {
		// Exif 1 p.30 (36 in PDF)             - DateTimeOriginal
		// Exif 3 p.9 (13 in PDF)              - exif:DateTimeOriginal
		d = e.Exif["DateTimeOriginal"]
	}
	if d == "" {
		// XMP 1 p.27 (35 in PDF)              - xmp:CreateDate ??? The digital or original.
		// XMP 2 p.32                          - photoshop:DateCreated
		d = e.XMP["DateCreated"]
	}

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

// Keywords returns the IPTC keywords value, or the XMP:Subject field. AFAICT
// there is no Exif tag for this.
//
// This field is available in our import template as 'Keywords'.
func (e exif) Keywords() string {
	var kw string
	// IPTC 3.1 p.2                            - Keywords
	// IPTC 6 p.31 (32 in PDF)                 - Keywords
	// IPTC 7 p.12                             - dc:subject
	kw = e.IPTC["Keywords"]
	// Exif 1 p.28 (34 in PDF) says USerComment may be used for Keywords, but
	// keywords isn't in Exif 3
	if kw == "" {
		// XMP 1 p.26 (34 in PDF)              - dc:subject
		kw = e.XMP["Subject"]
	}
	return kw
}

// HasKeywords returns true if Keywords is non-empty.
func (e exif) HasKeywords() bool {
	return e.Keywords() != ""
}

// Description returns the Description. Description has been mapped to
// IPTC.Caption-Abstract tag, the Exif.ImageDescription tag and also
// XMP.Description. So we try them in that order.
//
// This field is available in our import template as 'Description'.
func (e exif) Description() string {
	var d string
	// IPTC 3.1 p.2                            - Description
	// IPTC 6 p.39 (40 in PDF)                 - Caption/Abstract (/ not valid in field so -?)
	// IPTC 7 p.18                             - dc:description
	d = e.IPTC["Caption-Abstract"]
	if d == "" {
		// Exif 1 p.22 (28)                    - ImageDescription
		// Exif 3 p.6 (10)                     - dc:description
		d = e.Exif["ImageDescription"]
	}
	if d == "" {
		// XMP 1 p.25 (33)                     - dc:description
		d = e.XMP["Description"]
	}
	return d
}

// Has Description returns true if Description is non-empty.
func (e exif) HasDescription() bool {
	return e.Description() != ""
}

// NasaID tries to return some value for NASA ID. This is supposed to ALSO be
// the file name, but is also reportedly in IPTC's,JobID or the older
// IPTC.OriginalTransmissionReference.  We also may find it in Exif.ImageID or
// XMP.Title. So we try those and fall back to the File name sans extension.
//
// NB: It appears this is commonly misused, XMP.Title in particular. It seems
// these are often different than the filename or what is in the import
// template. This leads me to think we should just take the filename. However,
// we then have a problem of duplicate filenames for pre NSD-2822 assets.
// TODO: Decide what to do above.
//
// This field is available in our import template as 'NASA ID'.
func (e exif) NasaID() string {
	var id string
	// IPTC 3.1 p.2 contains a field 'Title' that may be used for this AFAICT.
	// IPTC 6 p.38 (39)                        - OriginalTransmissionReference
	// IPTC 7 p.17                             - photoshop:TransmissionReference
	id = e.IPTC["OriginalTransmissionReference"]
	if id == "" {
		// IPTC 1 p.7                          - Job Identifier
		// IPTC 3.1 p.2                        - Job ID
		// IPTC 5 p.15                         - JobID
		id = e.IPTC["JobID"]

	}
	if id == "" {
		// Exif 1 p. 45 (54)                   - ImageUniqueID
		// Exif 3 p.17 (21)                    - exif:ImageUniqueID
		id = e.Exif["ImageUniqueID"]
	}
	if id == "" {
		// XMP 1 p.27 (35)                     - xmp:Identifier
		// XMP 1 p.26 (34)                     - dc:identifier
		id = e.XMP["Identifier"]
	}
	if id == "" {
		// XMP 2 p.33                          - photoshop:TransmissionReference
		id = e.XMP["TransmissionReference"]
	}
	if id == "" {
		name := e.Data["FileName"]
		ext := filepath.Ext(name)
		id = name[0 : len(name)-len(ext)]
	}
	return id
}

// HasNasaID returns if exif.NasaId() returns a non empty string.
func (e exif) HasNasaID() bool {
	return e.NasaID() != ""
}

// Title tries to return a valid title for the asset. This has been mapped to
// IPTC.ObjectName or IPTC.Headline, but can also be XMP.Title. So we try
// them in that order. I don't see an equivalent in Exif.
//
// This field is availale in out ingestion template as 'Title'.
func (e exif) Title() string {
	var t string
	// IPTC 3.1 p.2 - Says Title is usually used for file name or id.
	// IPTC 6 p.26 (27)                         - ObjectName
	// IPTC 7 p.10                              - dc:title
	t = e.IPTC["ObjectName"]
	if t == "" {
		// IPTC 6 p.39 (40)                     - Headline
		// IPTC 7 p.17                          - photoshop:Headline
		t = e.IPTC["Headline"]
	}
	// No such Exif???
	if t == "" {
		// XMP 1 p.27                           - dc:title
		// XMP 2 p.32                           - photoshop:Headline
		t = e.XMP["Title"]
	}
	return t
}

// HasTitle returns  if exif.Title() returns a non empty string.
func (e exif) HasTitle() bool {
	return e.Title() != ""
}

// Location appears particularly difficult to map cleanly.
// IPTC:
// IPTCCore [IPTC 3.1 p.1] says the Location fields:
// Sublocation, City,State/Provoince, Country, ISO Country Code
// IPTC tags: IPTC.City, IPTC.Province-State, and IPTC.Country-Primary Location Name. There
// are also fields we might be able to use in XMP. It appears that what I see
// now in XMP are Creator City, Creator ..., But I am not sure that means the
// subject matter would share the info. So we are only doing IPTC here at this
// point. Exif appears not to support these tags but does have some GPS tags.
//
// These tags are collectively available in our ingestion template as 'Location'.
func (e exif) Location() string {
	var city, region, country string
	var addr []string
	// IPTC 6 p.37 (38)                        - City
	// IPTC 7 p.16                             - photoshop:City
	city = e.IPTC["City"]
	if city == "" {
		// XMP 2 p.32                          - photoshop:City
		city = e.XMP["City"]
	}
	if city != "" {
		addr = append(addr, city)
	}
	// IPTC 6 p.37 (38)                        - Province-State
	// IPTC 7 p.16                             - photoshop:State
	region = e.IPTC["Province-State"]
	if region == "" {
		// XMP 2 p.32                          - photoshop:State
		region = e.XMP["State"]
	}
	if region != "" {
		addr = append(addr, region)
	}
	// IPTC 6 p.38 (39)                        - Country-PrimaryLocationName
	// IPTC 7 p.17                             - photoshop:Country
	country = e.IPTC["Country-PrimaryLocationName"]
	if country == "" {
		// XMP 2 p.32                          - photoshop:Country
		country = e.XMP["Country"]
	}
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
// the MIME type and provides the first part if it is a valid mimeType. Here we
// are just gathering the type of the MIME Type and discarding the subtype.
// XMP supports this tag in the Dublin Core namespace.
//
// This tag is available in our ingestion template as 'Media Type'.
func (e exif) MediaType() string {
	var t string
	// XMP 1 p.26 (35)                         - dc:format
	t = e.XMP["Format"]
	if t == "" {
		// This just pulls from exiftool fileinfo.
		t = e.Data["MIMEType"]
	}
	if mimeTypes[t] {
		t = strings.Split(t, "/")[0]
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

// FileFormat returns the exiftool file format if the MIME type is in
// mimeTypes.  There is no 'file format' in the metadata standards that I can
// see. AFAICT the only place it is is the MIMEType (dc:format) above.
//
// This tag is available in our ingestion template as 'File Format'
func (e exif) FileFormat() string {
	if mimeTypes[e.Data["MIMEType"]] {
		// We pull this from the file data provided by exiftool
		return e.Data["FileType"]
	}
	return ""
}

// HasFileFormat returns if exif.FileType() returns a non-empty value.
func (e exif) HasFileFormat() bool {
	return e.FileFormat() != ""
}

// Photographer returns the IPTC By-line. It that fails it falls back to XMP
// Creator, then Exif Artist.
//
// This tag is available in our ingestion template as 'Photographer'.
func (e exif) Photographer() string {
	var p string
	// IPTC 6 p.36 (37)                        - By-line
	// IPTC 7 p.15                             - dc:creator
	p = e.IPTC["By-line"]
	if p == "" {
		// Exif 1 p.23 (29) 				   - Artist
		// Exif 2 p.40 (45) 				   - Artist
		// Exif 3 p.6  (10)  				   - dc:creator
		p = e.Exif["Artist"]
	}
	if p == "" {
		// XMP 1 p.25  (33)					   - dc:creator
		p = e.XMP["Artist"]
	}
	return p
}

// HasPhotographer returns if exif.Photographer returns a non-empty value.
func (e exif) HasPhotographer() bool {
	return e.Photographer() != ""
}

// MakeErrorRow creates a sequence suitable for the CSV output when an error
// has occured.
func (e exif) MakeErrorRow(c chan []string, p string, err error) {
	erow := []string{p, "Rejected", err.Error()}
	for _ = range csvHeader[3:] {
		erow = append(erow, "")
	}
	c <- erow
}

// MakeRow makes a row suitable for CSV output with the data from an individual
// file.
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
		e.NasaID(),
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

// parseDate, uh, parses the date from the string. If we decide we don't care
// about the subseconds we can force exiftool to output in ISO8601 format.
func parseDate(d string) (time.Time, error) {
	var (
		t      time.Time
		err    error
		format string
		// p      bool
	)
	d = strings.TrimSpace(d)
	// exiftool converts all dates to exif like format even if they are ISO
	// 8601. So this will all need redone if and when we get our own metadata.
	if strings.Contains(d, ".") {
		format = exifNanoDate
		if strings.Contains(d, "-") {
			format = exifNanoDateZone
			// p = true
		}
	} else {
		format = exifDate
		if strings.Contains(d, "-") {
			format = exifDateZone
		}
	}
	t, err = time.Parse(format, d)
	if err != nil {
		t, err = time.Parse(exifDateOnly, d)
	}
	return t, err

}

// getExifData gets the output of `exiftool p[ath]` and loads it into an exif struct.
func getExifData(p string) (exif, error) {
	exif := newExif()

	cmd := exec.Command("exiftool", "-G", "-s", "-a", p)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return exif, err
	}

	cmdOut := strings.Trim(out.String(), " \r\n")
	lines := strings.Split(cmdOut, "\n")

	for _, line := range lines {
		tk := strings.TrimSpace(line[0:48])
		t := strings.TrimSpace(tk[0:15])
		k := strings.TrimSpace(tk[16:])
		v := strings.TrimSpace(line[50:])

		switch {
		case t == "[EXIF]":
			exif.Exif[k] = v
		case t == "[IPTC]":
			exif.IPTC[k] = v
		case t == "[XMP]":
			exif.XMP[k] = v
		default:
			exif.Data[k] = v
		}
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

// makeWalker returns a function suitable for filepath.Walk. It walks the
// directory recursively and finds files that have relevant extensions. Which
// sends to the files channel.
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

// processFiles receives filepaths on the files channel. It then processes each
// file to extract metadata and make a 'row' for output. The main function
// launches one of these for each core the system is running on has.
func processFiles(files chan string, results chan []string, stats *statistics, wg *sync.WaitGroup) {
	defer wg.Done()
	var status, reason string
	for p := range files {
		e, err := getExifData(p)
		switch {
		case err != nil:
			atomic.AddInt32(&stats.Reject, 1)
			e.MakeErrorRow(results, p, err)
			if *verbose {
				log.Printf("Error processing %s: %s\n", p, err)
			}
		default:
			if e.HasDateCreated() && (e.HasKeywords() || e.HasDescription()) {
				atomic.AddInt32(&stats.Accept, 1)
				status = "Accepted"
				reason = ""
			} else {
				atomic.AddInt32(&stats.Reject, 1)
				status = "Incomplete"
				reason = "Minimum metadata not provided"
			}
			e.MakeRow(results, p, status, reason)
		}
	}
}

// make output receives rows on the c channel and writes them to the csv
// writer.
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
	var f *os.File
	if *output != "" {
		f, err = os.Create(*output)
		if err != nil {
			log.Fatalln("Error opening output file: ", err)
		}
		out = csv.NewWriter(f)
	} else {
		out = csv.NewWriter(os.Stdout)
	}
	err = out.Write(csvHeader)
	if err != nil {
		log.Printf("Error writing csvHeader: %s", err)
	}
	out.Flush()

	outgroup.Add(1)
	go func() {
		makeOutput(results, out, &outgroup)
	}()

	ingroup.Wait()
	close(results)
	outgroup.Wait()
	out.Flush()

	if *output != "" {
		err = f.Close()
		if err != nil {
			log.Printf("Error closing file %s: %s", f.Name(), err)
		}
	}

	log.Printf("\nTotal Found: %d\nRelevant Files: %d\nRejected Files: %d\nAccepted Files: %d\n",
		stats.Total, stats.Relevant, stats.Reject, stats.Accept)
}
