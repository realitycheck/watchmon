package watchmon

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/realitycheck/watchmon/yamlutil"
)

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
	Monitors []MonitorConfig
	Sources  []SourceConfig
	Graphs   []GraphConfig
}

type MonitorConfig struct {
	Id    string
	Title string
	Type  string
	Value MonitorValueConfig
}

type MonitorValueConfig struct {
	SourceId string `yaml:"sourceId"`
	RecordId string `yaml:"recordId"`
	Header   string
	Format   string
	Labels   []MonitorValueLabelConfig
}

type MonitorValueLabelConfig struct {
	Header string
	Format string
}

type SourceConfig struct {
	Id      string
	Command string
	Timeout time.Duration
	Output  SourceOutputConfig
}

type SourceOutputConfig struct {
	Parser  string
	Records []ParserRecordConfig
}

type ParserRecordConfig struct {
	Id                string
	FirstLineIsHeader bool `yaml:"firstLineIsHeader"`
	Header            []string
	ParserOptions     map[string]string `yaml:"parserOptions"`
}

type GraphConfig struct {
	Id            string
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

func MustLoadConfig(config string) AppConfig {
	appConfig, err := LoadConfig(config)
	if err != nil {
		panic(err)
	}
	return appConfig
}

func LoadConfig(filename string) (AppConfig, error) {
	var appConfig AppConfig
	fp, err := os.Open(filename)
	if err != nil {
		return appConfig, err
	}
	err = yaml.NewDecoder(fp).Decode(&appConfig)
	return appConfig, err
}

func ValidateConfig(filename string, schema string) {

}
