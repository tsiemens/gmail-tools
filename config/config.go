package config

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"gopkg.in/yaml.v2"

	"github.com/tsiemens/gmail-tools/util"
)

const (
	ConfigYamlFileName = "config.yaml"

	caseIgnore = "(?i)"
)

type Config struct {
	InterestingMessageQuery          string            `yaml:"InterestingMessageQuery"`
	AlwaysUninterestingLabelPatterns []string          `yaml:"AlwaysUninterestingLabelPatterns"`
	UninterestingLabelPatterns       []string          `yaml:"UninterestingLabelPatterns"`
	InterestingLabelPatterns         []string          `yaml:"InterestingLabelPatterns"`
	ApplyLabelToUninteresting        string            `yaml:"ApplyLabelToUninteresting"`
	ApplyLabelOnTouch                string            `yaml:"ApplyLabelOnTouch"`
	LabelColors                      map[string]string `yaml:"LabelColors"`

	AlwaysUninterLabelRegexps []*regexp.Regexp
	UninterLabelRegexps       []*regexp.Regexp
	InterLabelRegexps         []*regexp.Regexp
	ConfigFile                string
}

func loadConfig() *Config {
	confFname := util.RequiredHomeDirAndFile(util.UserAppDirName, ConfigYamlFileName)

	var confData []byte

	if _, err := os.Stat(confFname); err != nil {
		if os.IsNotExist(err) {
			// File does *not* exist
			_, err := os.Create(confFname)
			if err != nil {
				log.Fatalf("Failed to create config file: %v", err)
			}
			confData = make([]byte, 0)
		} else {
			// Schrodinger: file may or may not exist. See err for details.
			log.Fatalf("Failed to stat config file: %v", err)
		}
	}

	confData, err := ioutil.ReadFile(confFname)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	conf := &Config{}
	conf.ConfigFile = confFname
	err = yaml.Unmarshal(confData, conf)
	if err != nil {
		log.Fatalf("Could not unmarshal: %v", err)
	}
	util.Debugf("config: %+v\n", conf)

	checkLoadErr := func(e error) {
		if err != nil {
			log.Fatalf("Failed to load config: \"%s\"", err)
		}
	}

	for _, pat := range conf.AlwaysUninterestingLabelPatterns {
		re, err := regexp.Compile(caseIgnore + pat)
		checkLoadErr(err)
		conf.AlwaysUninterLabelRegexps = append(conf.AlwaysUninterLabelRegexps, re)
	}

	for _, pat := range conf.UninterestingLabelPatterns {
		re, err := regexp.Compile(caseIgnore + pat)
		checkLoadErr(err)
		conf.UninterLabelRegexps = append(conf.UninterLabelRegexps, re)
	}

	for _, pat := range conf.InterestingLabelPatterns {
		re, err := regexp.Compile(caseIgnore + pat)
		checkLoadErr(err)
		conf.InterLabelRegexps = append(conf.InterLabelRegexps, re)
	}
	return conf
}

var appConfig *Config

func AppConfig() *Config {
	if appConfig == nil {
		appConfig = loadConfig()
	}
	return appConfig
}
