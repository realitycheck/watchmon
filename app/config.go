package app

import (
	"embed"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/realitycheck/watchmon/pkg/yamlutil"
	log "github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed schemas/*.json
var schemas embed.FS

var AppConfigSchema string

func init() {
	bytes, err := schemas.ReadFile("schemas/config-schema.json")
	if err != nil {
		panic(err)
	}
	AppConfigSchema = string(bytes)
}

type dict map[string]interface{}

func (d *dict) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var res map[interface{}]interface{}
	if err := unmarshal(&res); err != nil {
		return err
	}
	*d = yamlutil.AnyMap(res)
	return nil
}

type AppConfig struct {
	Monitors []MonitorConfig `yaml:"monitors"`
	Sources  []SourceConfig  `yaml:"sources"`
	Graphs   []GraphConfig   `yaml:"graphs"`
}

type MonitorConfig struct {
	Id    string             `yaml:"id"`
	Title string             `yaml:"title"`
	Type  string             `yaml:"type"`
	Value MonitorValueConfig `yaml:"value"`
}

type MonitorValueConfig struct {
	SourceId string                    `yaml:"sourceId"`
	RecordId string                    `yaml:"recordId"`
	Header   string                    `yaml:"header"`
	Format   string                    `yaml:"format"`
	Labels   []MonitorValueLabelConfig `yaml:"labels"`
}

type MonitorValueLabelConfig struct {
	Header string `yaml:"header"`
	Format string `yaml:"format"`
}

type SourceConfig struct {
	Id      string             `yaml:"id"`
	Command string             `yaml:"command"`
	Timeout time.Duration      `yaml:"timeout"`
	Output  SourceOutputConfig `yaml:"output"`
}

type SourceOutputConfig struct {
	Parser  string               `yaml:"parser"`
	Records []ParserRecordConfig `yaml:"records"`
}

type ParserRecordConfig struct {
	Id                string            `yaml:"id"`
	FirstLineIsHeader bool              `yaml:"firstLineIsHeader"`
	Header            []string          `yaml:"header"`
	ParserOptions     map[string]string `yaml:"parserOptions"`
}

type GraphConfig struct {
	Id            string          `yaml:"id"`
	ChartDelay    int             `yaml:"chartDelay"`
	ChartOptions  dict            `yaml:"chartOptions"`
	SeriesOptions map[string]dict `yaml:"seriesOptions"`
	TimeOptions   map[string]dict `yaml:"timeOptions"`
}

func (c *AppConfig) MonitorsMap() map[string]*MonitorConfig {
	res := make(map[string]*MonitorConfig, len(c.Monitors))
	for _, m := range c.Monitors {
		m := m
		res[m.Id] = &m
	}
	return res
}

func (c AppConfig) Save(filename string) error {
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, bytes, 0777)
}

func LoadConfig(filename string) (AppConfig, error) {
	var appConfig AppConfig
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return appConfig, err
	}

	err = yaml.Unmarshal(bytes, &appConfig)
	if err == nil {
		var result *gojsonschema.Result
		var document dict
		err = yaml.Unmarshal(bytes, &document)
		if err == nil {
			result, err = gojsonschema.Validate(
				gojsonschema.NewStringLoader(AppConfigSchema),
				gojsonschema.NewGoLoader(document),
			)
			if err == nil && !result.Valid() {
				err = fmt.Errorf("%s: %s", filename, result.Errors()[0])
				logger := configLog("LoadConfig")
				if logger.Logger.IsLevelEnabled(log.DebugLevel) {
					for _, desc := range result.Errors() {
						logger.Errorf(" - %s\n", desc)
					}
				}
			}
		}
	}
	return appConfig, err
}
