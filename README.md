chkmd
=====

Reports on quality of embedded metadata for media assets to be imported.

Requirements
------------

Requires [go](https://golang.org/doc/install) to be installed.
Also expects [exiftool](http://www.sno.phy.queensu.ca/~phil/exiftool/) to be installed.

I used exiftool because it also works with video and audio files.

Installation
------------

`go get github.com/v-studios/chkmd`

Usage
-----

```shell
chkmd -h
Usage of ./chkmd:
  -c="dev-config.yaml": The config file to read from.
  -d="": The directory to process, recursively.
  -p=8: The number of processes to run.
```
Example
`chkmd -c myconfig.yaml -p 4 -d /path/to/media/assets`
