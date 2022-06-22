package app

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

var testConfig = AppConfig{
	Monitors: []MonitorConfig{
		{
			Id:    "arris_downstream_power",
			Title: "Downstream Frequency",
			Type:  "gauge",
			Value: MonitorValueConfig{
				"arris",
				"downstream",
				"power",
				"%f dBmV",
				[]MonitorValueLabelConfig{
					{Header: "dcid"}, {Header: "name"},
				},
			},
		},
		{
			Id:    "arris_downstream_snr",
			Title: "Downstream SNR",
			Type:  "gauge",
			Value: MonitorValueConfig{
				"arris",
				"downstream",
				"snr",
				"%f dB",
				[]MonitorValueLabelConfig{
					{Header: "dcid"}, {Header: "name"},
				},
			},
		},
	},
	Sources: []SourceConfig{
		{
			Id:      "arris",
			Command: "cat sample_source.html",
			Timeout: 5 * time.Second,
			Output: SourceOutputConfig{
				Parser: "htmlquery",
				Records: []ParserRecordConfig{
					{
						Id:                "downstream",
						FirstLineIsHeader: true,
						Header: []string{
							"name",
							"dcid",
							"freq",
							"power",
							"snr",
							"modulation",
							"octets",
							"correcteds",
							"uncorrectables",
						},
						ParserOptions: map[string]string{
							"format": "table",
							"path":   "//table[2]/tbody",
						},
					},
					{
						Id:                "upstream",
						FirstLineIsHeader: true,
						Header: []string{
							"name",
							"ucid",
							"freq",
							"power",
						},
						ParserOptions: map[string]string{
							"format": "table",
							"path":   "//table[4]/tbody",
						},
					},
				},
			},
		}, {
			Id:      "network",
			Command: "cat sample_source.csv",
			Timeout: 1 * time.Second,
			Output: SourceOutputConfig{
				Parser: "csv",
				Records: []ParserRecordConfig{
					{
						Id:                "downstream",
						FirstLineIsHeader: true,
						Header: []string{
							"signal",
							"ssid",
						},
						ParserOptions: map[string]string{
							"separator": ":",
						},
					},
				},
			},
		},
	},
	Graphs: []GraphConfig{
		{
			Id:         "arris_downstream_power",
			ChartDelay: 1000,
			ChartOptions: dict{
				"interpolation": "step",
			},
			SeriesOptions: map[string]dict{},
			TimeOptions:   map[string]dict{},
		},
	},
}

func Test_LoadConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "*.yaml")
	assert.NoError(t, err)

	defer os.Remove(f.Name())

	err = yaml.NewEncoder(f).Encode(testConfig)
	assert.NoError(t, err)

	err = f.Close()
	assert.NoError(t, err)

	got, err := LoadConfig(f.Name())
	assert.NoError(t, err)
	assert.Equal(t, got, testConfig)

	err = os.Remove(f.Name())
	assert.NoError(t, err)

	_, err = LoadConfig(f.Name())
	assert.Error(t, err)

}
