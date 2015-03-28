// Package main provides a program to check for AVAIL required metadata
package main

import (
	"flag"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v1"
)

var (
	cfgfile = flag.String("c", "dev-config.yaml", "The config file to read from.")

	mimeTypes = make(map[string]bool)
)

type config struct {
	MimeTypes []string `yaml:"mime_types"`
}

func readConfig(p string) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		log.Fatalf("Couldn't open config file: %s. Error: %s", p, err)
	}
	conf := config{}
	err = yaml.Unmarshal(b, &conf)
	if err != nil {
		log.Fatalf("Error parsing file %s: %s", p, err)
	}
	for _, t := range conf.MimeTypes {
		mimeTypes[t] = true
	}
}
