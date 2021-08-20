package config

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"gopkg.in/yaml.v2"

	"github.com/tsiemens/gmail-tools/prnt"
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
	ApplyLabelOnTouch                string            `yaml:"ApplyLabelOnTouch"`
	LabelColors                      map[string]string `yaml:"LabelColors"`
	Aliases                          map[string]string `yaml:"Aliases"`

	AlwaysUninterLabelRegexps []*regexp.Regexp
	UninterLabelRegexps       []*regexp.Regexp
	InterLabelRegexps         []*regexp.Regexp
	ConfigFile                string
}

// LoadConfigInto loads the central config file contents into confOut object.
// If the file does not exist, it is created. If it cannot be created or read,
// the process will be terminated.
// Returns the name of the loaded config file.
//
// confOut : A config object marked up for yaml unmarshaling.
//           This may be any subset of the config file, allowing for use by
//           plugins to add extra fields into the config file for their own use.
func LoadConfigInto(confOut interface{}) string {
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

	err = yaml.Unmarshal(confData, confOut)
	if err != nil {
		log.Fatalf("Could not unmarshal: %v", err)
	}
	util.Debugf("config: %+v\n", confOut)

	return confFname
}

func UserFriendlyMustCompile(pattern string, attrName string, configComponent string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		prnt.StderrLog.Fatalf("Error loading %s config attribute %s '%s': %v\n",
			configComponent, attrName, pattern, err)
	}
	return re
}

// May be called at any time (all dependencies are static)
func loadConfig() *Config {
	conf := &Config{}
	confFname := LoadConfigInto(conf)
	conf.ConfigFile = confFname

	comp := "main"
	for _, pat := range conf.AlwaysUninterestingLabelPatterns {
		re := UserFriendlyMustCompile(
			caseIgnore+pat, "AlwaysUninterestingLabelPatterns", comp)
		conf.AlwaysUninterLabelRegexps = append(conf.AlwaysUninterLabelRegexps, re)
	}

	for _, pat := range conf.UninterestingLabelPatterns {
		re := UserFriendlyMustCompile(caseIgnore+pat, "UninterestingLabelPatterns", comp)
		conf.UninterLabelRegexps = append(conf.UninterLabelRegexps, re)
	}

	for _, pat := range conf.InterestingLabelPatterns {
		re := UserFriendlyMustCompile(caseIgnore+pat, "InterestingLabelPatterns", comp)
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
