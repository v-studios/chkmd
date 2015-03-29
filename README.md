chkmd
=====

Reports on quality of embedded metadata for media assets to be imported.

Requirements
------------

Requires go to be installed.

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
