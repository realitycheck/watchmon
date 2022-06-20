package watchmon

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_Monitor_write_data(t *testing.T) {
	r := record{
		"correcteds":     "29883",
		"dcid":           "76",
		"freq":           "138.00 MHz",
		"modulation":     "256QAM",
		"name":           "Downstream 4",
		"octets":         "991393690",
		"power":          "2.33 dBmV",
		"snr":            "37.94 dB",
		"uncorrectables": "11059",
	}

	tests := []struct {
		name       string
		v          MonitorValueConfig
		wantLabels []string
		wantValue  float64
	}{
		{
			"Check empty value",
			MonitorValueConfig{},
			[]string{},
			0,
		}, {
			"Check empty value format",
			MonitorValueConfig{
				Header: "correcteds",
			},
			[]string{},
			0,
		}, {
			"Check bad value format",
			MonitorValueConfig{
				Header: "correcteds",
				Format: "%d",
			},
			[]string{},
			0,
		}, {
			"Check correct value format",
			MonitorValueConfig{
				Header: "correcteds",
				Format: "%f",
			},
			[]string{},
			29883,
		}, {
			"Check correct value format with labels",
			MonitorValueConfig{
				Header: "freq",
				Format: "%f MHz",
				Labels: []MonitorValueLabelConfig{
					{Header: "dcid"}, {Header: "name"},
				},
			},
			[]string{"76", "Downstream 4"},
			138,
		}, {
			"Check correct value format with labels(format)",
			MonitorValueConfig{
				Header: "modulation",
				Format: "%fQAM",
				Labels: []MonitorValueLabelConfig{
					{Header: "power", Format: "%s dBmV"},
				},
			},
			[]string{"2.33"},
			256,
		},
	}
	for _, tt := range tests {
		m := Monitor{
			c: MonitorConfig{
				Value: tt.v,
			},
			gauge: prom.NewGaugeVec(
				prom.GaugeOpts{
					Name: "m.c.Id",
					Help: "m.c.Title",
				}, labelNames(tt.v.Labels)),
		}
		t.Run(tt.name, func(t *testing.T) {
			gotLabels, gotValue := m.data(r)
			assert.Equal(t, tt.wantLabels, gotLabels)
			assert.Equal(t, tt.wantValue, gotValue)

			m.write(r) // nothing useful to assert here right now
		})

	}
}

func Test_Source_pull_output(t *testing.T) {
	sample := `
	0:s0
	255:s1
	127:s2`

	tests := []struct {
		name        string
		command     *ShellCommand
		m           *mock.Mock
		wantErr     string
		wantRecords records
	}{
		{
			name:    "error: undefined command",
			wantErr: "pull: undefined command",
		},
		{
			name: "error: timeout",
			command: &ShellCommand{
				Cmd: fmt.Sprintf("echo '%s'", sample),
			},
			wantErr: "context deadline exceeded",
		},
		{
			name: "ok",
			command: &ShellCommand{
				Cmd:     fmt.Sprintf("echo '%s'", sample),
				Timeout: 1 * time.Second,
			},
			wantRecords: records{
				"something": []record{},
			},
		},
		{
			name: "error: parse error",
			command: &ShellCommand{
				Cmd:     fmt.Sprintf("echo '%s'", sample),
				Timeout: 1 * time.Second,
			},
			wantRecords: records{
				"something": []record{},
			},
			wantErr: "some parsing error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Source{
				command: tt.command,
				parse: func(r io.Reader, b records) error {
					assert.Equal(t, r, strings.NewReader(sample+"\n"))
					assert.Equal(t, b, records{})
					if tt.wantErr != "" {
						return fmt.Errorf("%s", tt.wantErr)
					}

					for k, v := range tt.wantRecords {
						b[k] = v
					}
					return nil
				},
			}

			if tt.m != nil {
				tt.m.On("1")
			}

			got, err := s.pull()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantRecords, got)
			}

		})
	}
}

func Test_OutputParser_parseCSV(t *testing.T) {
	sample := `
	0:s0
	255:s1
	127:s2`

	tests := []struct {
		name    string
		records []ParserRecordConfig
		want    records
		wantErr string
	}{
		{
			"test #1 (empty)",
			[]ParserRecordConfig{},
			records{},
			"",
		},
		{
			"test #2 (correct)",
			[]ParserRecordConfig{
				{
					Id:     "wifi",
					Header: []string{"signal", "ssid"},
					ParserOptions: map[string]string{
						"separator": ":",
					},
				},
			},
			records{
				"wifi": []record{
					{"signal": "0", "ssid": "s0"},
					{"signal": "255", "ssid": "s1"},
					{"signal": "127", "ssid": "s2"},
				},
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := OutputParser{tt.records}
			got := make(records)
			err := p.parseCSV(strings.NewReader(sample), got)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_OutputParser_parseHTMLQuery_table(t *testing.T) {
	sample := `
	<table>
		<tbody>
			<tr>
				<td></td>
				<td>DCID</td>
				<td>Freq</td>
				<td>Power</td>
			</tr>
			<tr>
				<td>Downstream 1</td>
				<td>73</td>
				<td>114.00 MHz</td>
				<td>0.82 dBmV</td>
			</tr>
			<tr>
				<td>Downstream 2</td>
				<td>74</td>
				<td>122.00 MHz</td>
				<td>2.70 dBmV</td>
			</tr>
		</tbody>
	</table>
	<table>
		<tbody>
			<tr>
				<td></td>
				<td>UCID</td>
				<td>Freq</td>
			</tr>
			<tr></tr>
			<tr>
				<td>Upstream 1</td>
				<td>5</td>
				<td>36.00 MHz</td>
			</tr>
		<tbody>
	</table>`

	tests := []struct {
		name    string
		records []ParserRecordConfig
		want    records
		wantErr string
	}{
		{
			"test #1 (empty)",
			[]ParserRecordConfig{},
			records{},
			"",
		}, {
			"test #2 (bad parser format)",
			[]ParserRecordConfig{
				{
					ParserOptions: map[string]string{},
				},
			},
			records{},
			"parseHTMLQuery: invalid parser option 'format': map[]",
		}, {
			"test #3 (bad parser path)",
			[]ParserRecordConfig{
				{
					ParserOptions: map[string]string{
						"format": "table",
					},
				},
			},
			records{},
			"parseHTMLQuery: invalid parser option 'path': map[format:table]",
		}, {
			"test #4 (correct record)",
			[]ParserRecordConfig{
				{
					Id:                "downstream",
					FirstLineIsHeader: true,
					ParserOptions: map[string]string{
						"format": "table",
						"path":   "//table[1]/tbody",
					},
					Header: []string{"name", "dcid", "freq", "power"},
				},
			},
			records{
				"downstream": []record{
					{
						"dcid":  "73",
						"freq":  "114.00 MHz",
						"name":  "Downstream 1",
						"power": "0.82 dBmV",
					}, {
						"dcid":  "74",
						"freq":  "122.00 MHz",
						"name":  "Downstream 2",
						"power": "2.70 dBmV",
					},
				},
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := OutputParser{tt.records}
			got := make(records)
			err := p.parseHTMLQuery(strings.NewReader(sample), got)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
