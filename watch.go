package watchmon

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sync"

	"os/exec"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"

	prom "github.com/prometheus/client_golang/prometheus"
)

type (
	table   [][]string
	record  map[string]string
	records map[string][]record
)

type WatchService struct {
	monitors []*Monitor
	sources  []*Source
}

type Monitor struct {
	c     MonitorConfig
	gauge *prom.GaugeVec
}

type Source struct {
	c       SourceConfig
	command *ShellCommand
	output  *OutputParser
	parse   func(r io.Reader, b records) error
}

type ShellCommand struct {
	Cmd     string
	Timeout time.Duration
}

type OutputParser struct {
	records []ParserRecordConfig
}

func initWatchService(app *Application, config AppConfig) {
	app.ws = &WatchService{
		make([]*Monitor, len(config.Monitors)),
		make([]*Source, len(config.Sources)),
	}

	for i, c := range config.Monitors {
		app.ws.monitors[i] = &Monitor{c: c}
		m := app.ws.monitors[i]

		if m.c.Value.Format == "" {
			m.c.Value.Format = "%f"
		}

		switch m.c.Type {
		case "gauge":
			m.gauge = prom.NewGaugeVec(
				prom.GaugeOpts{
					Name: m.c.Id,
					Help: m.c.Title,
				}, labelNames(m.c.Value.Labels))
			prom.MustRegister(m.gauge)
		}
	}

	for i, c := range config.Sources {
		app.ws.sources[i] = &Source{c: c}
		s := app.ws.sources[i]

		s.command = &ShellCommand{
			Cmd:     s.c.Command,
			Timeout: s.c.Timeout,
		}
		s.output = &OutputParser{
			s.c.Output.Records,
		}
		switch s.c.Output.Parser {
		case "csv":
			s.parse = s.output.parseCSV
		case "htmlquery":
			s.parse = s.output.parseHTMLQuery
		}
	}
}

func labelNames(ll []MonitorValueLabelConfig) []string {
	labelNames := make([]string, len(ll))
	for i, l := range ll {
		labelNames[i] = l.Header
	}
	return labelNames
}

func (ws *WatchService) Start(delay time.Duration) {
	type SourcesData struct {
		data    *sync.Map
		updated time.Time
	}
	sourcesData := make(chan SourcesData)
	var latest time.Time

	for {
		select {
		case <-time.After(delay):
			go func() {
				updated := time.Now()
				data := &sync.Map{}
				wg := sync.WaitGroup{}
				wg.Add(len(ws.sources))
				for _, source := range ws.sources {
					go func(s *Source) {
						records, err := s.pull()
						if err != nil {
							watchLog("source.pull").WithError(err).WithField("source", s.c.Id).Error("Pull failure")
						} else {
							data.Store(s.c.Id, records)
						}
						wg.Done()
					}(source)
				}
				wg.Wait()
				watchLog("source.pull").Debugf("Push source data")
				sourcesData <- SourcesData{data, updated}
			}()
		case sources := <-sourcesData:
			if time.Since(latest) < time.Since(sources.updated) {
				watchLog("monitor.write").WithField(
					"latest", time.Since(latest),
				).WithField(
					"received", time.Since(sources.updated),
				).Debugf("Stale data ignored")
				break
			}
			go func() {
				defer func() {
					latest = sources.updated
				}()
				for _, m := range ws.monitors {
					value, ok := sources.data.Load(m.c.Value.SourceId)
					if ok {
						records, ok := value.(records)[m.c.Value.RecordId]
						if ok {
							for _, r := range records {
								m.write(r)
							}
						}
					}
				}
			}()
		}
	}
}

func (m *Monitor) write(r record) {
	labels, val := m.data(r)
	m.gauge.WithLabelValues(labels...).Set(val)
	watchLog("monitor.write").WithField("metric", m.c.Id).WithField("record", r).Debugf("Written data: %v %f", labels, val)
}

func (m *Monitor) data(r record) ([]string, float64) {
	v, ok := r[m.c.Value.Header]
	var val float64
	if ok {
		fmt.Sscanf(v, m.c.Value.Format, &val)
	}
	labels := make([]string, len(m.c.Value.Labels))
	for i, k := range m.c.Value.Labels {
		v, ok = r[k.Header]
		if ok {
			if k.Format != "" {
				fmt.Sscanf(v, k.Format, &labels[i])
			} else {
				labels[i] = v
			}
		}
	}
	return labels, val
}

func (s *Source) pull() (records, error) {
	if s.command == nil {
		return nil, fmt.Errorf("pull: undefined command")
	}
	output, err := s.command.output()
	if err != nil {
		return nil, err
	}
	res := make(records)
	err = s.parse(strings.NewReader(string(output)), res)
	if err != nil {
		return nil, err
	}

	watchLog("source.pull").Debugf("Parsed records: %+v", res)
	return res, nil
}

func (c *ShellCommand) output() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	res, err := exec.CommandContext(ctx, "sh", "-c", c.Cmd).CombinedOutput()
	if err != nil {
		return nil, err
	}

	watchLog("shell.output").Tracef("%s", res)
	return res, nil
}

func (p *OutputParser) parseCSV(r io.Reader, b records) error {
	csvr := csv.NewReader(r)
	csvr.Comma = ':'
	csvr.TrimLeadingSpace = true

	res, err := csvr.ReadAll()
	if err != nil {
		return err
	}
	watchLog("parser.csv").Debugf("Data: %+v", res)

	for i := 0; i < len(p.records); i++ {
		r := p.records[i]
		b[r.Id] = table(res).zip(r.Header, r.FirstLineIsHeader)
	}
	return nil
}

func (p *OutputParser) parseHTMLQuery(r io.Reader, b records) error {
	doc, err := html.Parse(r)
	if err != nil {
		return err
	}
	for i := 0; i < len(p.records); i++ {
		r := p.records[i]
		var table table
		switch r.ParserOptions["format"] {
		case "table":
			table, err = p.parseFormatTable(&r, doc)
			if err != nil {
				return fmt.Errorf("parseHTMLQuery: %v", err)
			}
		default:
			return fmt.Errorf("parseHTMLQuery: invalid parser option 'format': %+v", r.ParserOptions)
		}
		b[r.Id] = table.zip(r.Header, r.FirstLineIsHeader)
	}
	return nil
}

func (p *OutputParser) parseFormatTable(r *ParserRecordConfig, doc *html.Node) (table, error) {
	path, ok := r.ParserOptions["path"]
	if !ok {
		return nil, fmt.Errorf("invalid parser option 'path': %+v", r.ParserOptions)
	}
	tr := htmlquery.Find(htmlquery.FindOne(doc, path), "/tr[td]")
	res := make(table, len(tr))
	for i, r := range tr {
		td := htmlquery.Find(r, "/td")
		res[i] = make([]string, len(td))
		for j, d := range td {
			res[i][j] = htmlquery.InnerText(d)
		}
	}
	watchLog("parser.htmlquery").Debugf("Data: %+v", res)
	return res, nil
}

func (t table) zip(header []string, skipFirstLine bool) []record {
	res := make([]record, len(t))
	for i, r := range t {
		res[i] = make(record)
		for j := 0; j < len(header); j++ {
			res[i][header[j]] = r[j]
		}
	}
	if skipFirstLine {
		res = res[1:]
	}
	return res
}
