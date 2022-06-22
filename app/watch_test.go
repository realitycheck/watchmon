package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

type (
	testMetric struct {
		written []metric
		err     error
	}

	testCommand struct {
		res string
		err error
	}

	testParser struct {
		res records
		err error
	}
)

func (m *testMetric) Write(monitor *Monitor, value metric) error {
	m.written = append(m.written, value)
	return m.err
}

func (c *testCommand) Execute(source *Source) ([]byte, error) {
	return []byte(c.res), c.err
}

func (p *testParser) Parse(source *Source, reader io.Reader) (records, error) {
	return p.res, p.err
}

func Test_Monitor_push(t *testing.T) {
	rr := []record{
		{
			"correcteds":     "29883",
			"dcid":           "76",
			"freq":           "138.00 MHz",
			"modulation":     "256QAM",
			"name":           "Downstream 4",
			"octets":         "991393690",
			"power":          "2.33 dBmV",
			"snr":            "37.94 dB",
			"uncorrectables": "11059",
		},
		{
			"correcteds":     "29882",
			"dcid":           "75",
			"freq":           "118.00 MHz",
			"modulation":     "256QAM",
			"name":           "Downstream 3",
			"octets":         "919393690",
			"power":          "2.35 dBmV",
			"snr":            "38.74 dB",
			"uncorrectables": "1059",
		},
	}

	tests := []struct {
		name string
		v    MonitorValueConfig
		want []metric
	}{
		{
			"Check empty value",
			MonitorValueConfig{},
			[]metric{
				{[]string{}, 0},
				{[]string{}, 0},
			},
		}, {
			"Check empty value format",
			MonitorValueConfig{
				Header: "correcteds",
			},
			[]metric{
				{[]string{}, 0},
				{[]string{}, 0},
			},
		}, {
			"Check bad value format",
			MonitorValueConfig{
				Header: "correcteds",
				Format: "%d",
			},
			[]metric{
				{[]string{}, 0},
				{[]string{}, 0},
			},
		}, {
			"Check correct value format",
			MonitorValueConfig{
				Header: "correcteds",
				Format: "%f",
			},
			[]metric{
				{[]string{}, 29883},
				{[]string{}, 29882},
			},
		}, {
			"Check correct value format with labels",
			MonitorValueConfig{
				Header: "freq",
				Format: "%f MHz",
				Labels: []MonitorValueLabelConfig{
					{Header: "dcid"}, {Header: "name"},
				},
			},
			[]metric{
				{[]string{"76", "Downstream 4"}, 138},
				{[]string{"75", "Downstream 3"}, 118},
			},
		}, {
			"Check correct value format with labels(format)",
			MonitorValueConfig{
				Header: "modulation",
				Format: "%fQAM",
				Labels: []MonitorValueLabelConfig{
					{Header: "power", Format: "%s dBmV"},
				},
			},
			[]metric{
				{[]string{"2.33"}, 256},
				{[]string{"2.35"}, 256},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := &testMetric{}
			m := Monitor{
				c: MonitorConfig{
					Value: tt.v,
				},
				gauge: prom.NewGaugeVec(
					prom.GaugeOpts{
						Name: "m.c.Id",
						Help: "m.c.Title",
					}, labelNames(tt.v.Labels)),
				metric: metric,
			}

			m.push(rr)

			assert.Equal(t, metric.written, tt.want)
		})

	}
}

func Test_Source_pull(t *testing.T) {
	sample := `
	0:s0
	255:s1
	127:s2`

	tests := []struct {
		name        string
		command     Command
		parser      Parser
		wantErr     string
		wantRecords records
	}{
		{
			name:    "error: undefined command",
			wantErr: "source: undefined command",
		},
		{
			name: "error: timeout",
			command: &testCommand{
				err: fmt.Errorf("context deadline exceeded"),
			},
			wantErr: "context deadline exceeded",
		},
		{
			name:    "ok",
			command: &testCommand{res: sample},
			parser: &testParser{
				res: records{
					"something": []record{},
				},
			},
			wantRecords: records{
				"something": []record{},
			},
		},
		{
			name:    "error: parse error",
			command: &testCommand{res: sample},
			parser: &testParser{
				err: fmt.Errorf("some parsing error"),
			},
			wantErr: "some parsing error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Source{
				command: tt.command,
				parser:  tt.parser,
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

func Test_csvParser_Parse(t *testing.T) {
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
			s := &Source{}
			s.c.Output.Records = tt.records
			p := csvParser{}
			got, err := p.Parse(s, strings.NewReader(sample))
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_htmlqueryParser_Parse(t *testing.T) {
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
			nil,
			"htmlqueryParser: invalid parser option 'format': map[]",
		}, {
			"test #3 (bad parser path)",
			[]ParserRecordConfig{
				{
					ParserOptions: map[string]string{
						"format": "table",
					},
				},
			},
			nil,
			"htmlqueryParser: invalid parser option 'path': map[format:table]",
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
			s := &Source{}
			s.c.Output.Records = tt.records
			p := htmlqueryParser{}
			got, err := p.Parse(s, strings.NewReader(sample))
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_shellCommand_Execute(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		timeout time.Duration
		want    []byte
		wantErr string
	}{
		{
			name:    "empty",
			wantErr: "context deadline exceeded",
		},
		{
			name:    "echo",
			cmd:     "echo test",
			timeout: 1 * time.Second,
			want:    []byte("test\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			s.c.Command = tt.cmd
			s.c.Timeout = tt.timeout
			c := shellCommand{}
			got, err := c.Execute(s)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_gaugeMetric_Write(t *testing.T) {
	m := &Monitor{
		gauge: prom.NewGaugeVec(
			prom.GaugeOpts{Name: "test"}, []string{"a", "b"},
		),
	}
	g := &gaugeMetric{}
	v := metric{[]string{"A", "B"}, 123}

	err := g.Write(m, v)
	assert.NoError(t, err)

	gauge, err := m.gauge.GetMetricWithLabelValues(v.labels...)
	assert.NoError(t, err)

	written := &dto.Metric{}
	err = gauge.Write(written)
	assert.NoError(t, err)
	assert.Equal(t, v.value, *written.Gauge.Value)
	assert.Equal(t, 2, len(written.Label))
}

func Test_WatchService_Start(t *testing.T) {
	tests := []struct {
		name        string
		run         func(m *Monitor, s *Source)
		testMetric  *testMetric
		testCommand *testCommand
		testParser  *testParser
	}{
		{
			name: "start and stop",
			run: func(m *Monitor, s *Source) {
				ws := WatchService{[]*Monitor{m}, []*Source{s}}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
				defer cancel()

				ws.Start(ctx, 1*time.Millisecond)
			},
			testMetric:  &testMetric{},
			testCommand: &testCommand{},
			testParser:  &testParser{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(
				&Monitor{
					metric: tt.testMetric,
				}, &Source{
					command: tt.testCommand,
					parser:  tt.testParser,
				})
		})
	}
}
