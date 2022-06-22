package app

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
	metric  struct {
		labels []string
		value  float64
	}

	Metric interface {
		Write(monitor *Monitor, m metric) error
	}

	Parser interface {
		Parse(source *Source, r io.Reader) (records, error)
	}

	Command interface {
		Execute(source *Source) ([]byte, error)
	}

	gaugeMetric     struct{}
	csvParser       struct{}
	htmlqueryParser struct{}
	shellCommand    struct{}
)

type WatchService struct {
	monitors []*Monitor
	sources  []*Source
}

type Monitor struct {
	c      MonitorConfig
	gauge  *prom.GaugeVec
	metric Metric
}

type Source struct {
	c       SourceConfig
	command Command
	parser  Parser
}

func NewWatchService(config AppConfig) *WatchService {
	ws := &WatchService{
		make([]*Monitor, len(config.Monitors)),
		make([]*Source, len(config.Sources)),
	}

	for i, c := range config.Monitors {
		ws.monitors[i] = &Monitor{c: c}
		m := ws.monitors[i]

		if m.c.Value.Format == "" {
			m.c.Value.Format = "%f"
		}

		if m.c.Type == "" {
			m.c.Type = "gauge"
		}

		switch m.c.Type {
		case "gauge":
			m.gauge = prom.NewGaugeVec(
				prom.GaugeOpts{
					Name: m.c.Id,
					Help: m.c.Title,
				}, labelNames(m.c.Value.Labels))
			prom.MustRegister(m.gauge)
			m.metric = &gaugeMetric{}
		}
	}

	for i, c := range config.Sources {
		ws.sources[i] = &Source{c: c}
		s := ws.sources[i]

		s.command = &shellCommand{}
		switch s.c.Output.Parser {
		case "csv":
			s.parser = &csvParser{}
		case "htmlquery":
			s.parser = &htmlqueryParser{}
		}
	}
	return ws
}

func labelNames(ll []MonitorValueLabelConfig) []string {
	labelNames := make([]string, len(ll))
	for i, l := range ll {
		labelNames[i] = l.Header
	}
	return labelNames
}

func (ws *WatchService) Start(ctx context.Context, refresh time.Duration) error {
	type SourcesData struct {
		data    *sync.Map
		updated time.Time
	}
	sourcesData := make(chan SourcesData)
	latest := struct {
		mu *sync.Mutex
		t  time.Time
	}{
		mu: &sync.Mutex{},
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(refresh):
			go func() {
				updated := time.Now()
				data := &sync.Map{}
				wg := sync.WaitGroup{}
				wg.Add(len(ws.sources))
				for _, source := range ws.sources {
					go func(s *Source) {
						records, err := s.pull()
						if err != nil {
							watchLog("WatchService").WithError(err).WithField("source", s.c.Id).Warn("Source refresh failure")
						} else {
							data.Store(s.c.Id, records)
						}
						wg.Done()
					}(source)
				}
				wg.Wait()
				sourcesData <- SourcesData{data, updated}
			}()
		case sources := <-sourcesData:
			latest.mu.Lock()
			t := latest.t
			latest.mu.Unlock()
			if time.Since(t) < time.Since(sources.updated) {
				watchLog("WatchService").WithField(
					"latest", time.Since(t),
				).WithField(
					"received", time.Since(sources.updated),
				).Debugf("Stale source data received: ignore")
				break
			}
			go func() {
				defer func() {
					latest.mu.Lock()
					defer latest.mu.Unlock()
					latest.t = sources.updated
				}()
				for _, m := range ws.monitors {
					value, ok := sources.data.Load(m.c.Value.SourceId)
					if ok {
						records, ok := value.(records)[m.c.Value.RecordId]
						if ok {
							m.push(records)
						}
					}
				}
			}()
		}
	}
}

func (g *gaugeMetric) Write(monitor *Monitor, m metric) error {
	monitor.gauge.WithLabelValues(m.labels...).Set(m.value)
	watchLog("gaugeMetric").WithField("metric", monitor.c.Id).Debugf("Written: %v %f", m.labels, m.value)
	return nil
}

func (m *Monitor) push(rr []record) {
	for _, r := range rr {
		m.metric.Write(m, r.value(m.c.Value))
	}
}

func (s *Source) pull() (records, error) {
	if s.command == nil {
		return nil, fmt.Errorf("source: undefined command")
	}
	output, err := s.command.Execute(s)
	if err != nil {
		return nil, err
	}
	res, err := s.parser.Parse(s, strings.NewReader(string(output)))
	if err != nil {
		return nil, err
	}
	watchLog("Source").Debugf("Parsed records: %+v", res)
	return res, nil
}

func (*shellCommand) Execute(s *Source) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.c.Timeout)
	defer cancel()

	res, err := exec.CommandContext(ctx, "sh", "-c", s.c.Command).CombinedOutput()
	if err != nil {
		watchLog("shellCommand").Debugf("%s", res)
		return nil, err
	}

	watchLog("shellCommand").Tracef("%s", res)
	return res, nil
}

func (*csvParser) Parse(s *Source, r io.Reader) (records, error) {
	csvr := csv.NewReader(r)
	csvr.Comma = ':'
	csvr.TrimLeadingSpace = true

	data, err := csvr.ReadAll()
	if err != nil {
		return nil, err
	}
	watchLog("csvParser").Debugf("Parsing data: %+v", data)
	res := make(records, len(s.c.Output.Records))
	for i := 0; i < len(s.c.Output.Records); i++ {
		r := s.c.Output.Records[i]
		res[r.Id] = table(data).zip(r.Header, r.FirstLineIsHeader)
	}
	return res, nil
}

func (p *htmlqueryParser) Parse(s *Source, r io.Reader) (records, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	res := make(records, len(s.c.Output.Records))
	for i := 0; i < len(s.c.Output.Records); i++ {
		r := s.c.Output.Records[i]
		var t table
		switch r.ParserOptions["format"] {
		case "table":
			t, err = p.parseFormatTable(&r, doc)
			if err != nil {
				return nil, fmt.Errorf("htmlqueryParser: %v", err)
			}
		default:
			return nil, fmt.Errorf("htmlqueryParser: invalid parser option 'format': %+v", r.ParserOptions)
		}
		res[r.Id] = t.zip(r.Header, r.FirstLineIsHeader)
	}
	return res, nil
}

func (p *htmlqueryParser) parseFormatTable(r *ParserRecordConfig, doc *html.Node) (table, error) {
	path, ok := r.ParserOptions["path"]
	if !ok {
		return nil, fmt.Errorf("invalid parser option 'path': %+v", r.ParserOptions)
	}
	tr := htmlquery.Find(htmlquery.FindOne(doc, path), "/tr[td]")
	watchLog("htmlqueryParser").Debugf("Parsing data: %+v", tr)
	res := make(table, len(tr))
	for i, r := range tr {
		td := htmlquery.Find(r, "/td")
		res[i] = make([]string, len(td))
		for j, d := range td {
			res[i][j] = htmlquery.InnerText(d)
		}
	}
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

func (r record) value(c MonitorValueConfig) metric {
	v, ok := r[c.Header]
	var val float64
	if ok {
		fmt.Sscanf(v, c.Format, &val)
	}
	ll := make([]string, len(c.Labels))
	for i, k := range c.Labels {
		v, ok = r[k.Header]
		if ok {
			if k.Format != "" {
				fmt.Sscanf(v, k.Format, &ll[i])
			} else {
				ll[i] = v
			}
		}
	}
	return metric{ll, val}
}
