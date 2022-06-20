package watchmon

import (
	"embed"
	"encoding/json"
	"net/http"
	"strings"
	"text/template"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed templates static
var content embed.FS
var templates *template.Template

func init() {
	templates = template.Must(
		template.New("content").Funcs(template.FuncMap{}).ParseFS(content, "templates/*.tmpl"),
	)
}

type HTTPService struct {
	mux *http.ServeMux

	configData dict

	templatesData map[string]dict
}

func initHTTPService(app *Application, config AppConfig) {
	app.hs = &HTTPService{mux: http.NewServeMux()}

	app.hs.configData = makeConfigData(config)
	app.hs.templatesData = makeTemplatesData(config)

	app.hs.mux.Handle("/", http.HandlerFunc(app.hs.serveRoot))
	app.hs.mux.Handle("/config.json", http.HandlerFunc(app.hs.serveConfigData))
	app.hs.mux.Handle("/metrics", promhttp.Handler())
	app.hs.mux.Handle("/static/", http.FileServer(http.FS(content)))
}

func (hs *HTTPService) serveRoot(w http.ResponseWriter, r *http.Request) {
	res := strings.TrimLeft(r.URL.Path, "/")
	if len(res) == 0 {
		res = "index.html"
	}
	tmpl := templates.Lookup(res + ".tmpl")
	if tmpl == nil {
		http.NotFound(w, r)
		return
	}
	if err := tmpl.Execute(w, hs.templatesData[res]); err != nil {
		httpLog("index.html").WithError(err).Error("can't execute template")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (hs *HTTPService) serveConfigData(w http.ResponseWriter, r *http.Request) {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	if err := e.Encode(hs.configData); err != nil {
		httpLog("config.json").WithError(err).Error("can't encode data")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func makeTemplatesData(config AppConfig) map[string]dict {
	type Group struct {
		Title    string
		Monitors []MonitorConfig
	}

	groups := map[string]int{} // group ordering
	getGroupId := func(g string) int {
		i, ok := groups[g]
		if !ok {
			i = len(groups)
			groups[g] = i
		}
		return i
	}

	data := map[int]*Group{}
	for _, m := range config.Monitors {
		groupId := getGroupId(m.Value.SourceId + " " + m.Value.RecordId)
		var group *Group
		group, ok := data[groupId]
		if !ok {
			group = &Group{
				Title:    strings.Title(m.Value.RecordId),
				Monitors: []MonitorConfig{},
			}
			data[groupId] = group
		}
		group.Monitors = append(group.Monitors, m)
	}

	canvas := make([]*Group, len(groups))
	for i := 0; i < len(groups); i++ {
		canvas[i] = data[i]
	}

	return map[string]dict{
		"index.html": {
			"Canvas": canvas,
		},
	}
}

func makeConfigData(config AppConfig) dict {
	graphs := make(dict, len(config.Graphs))
	monitors := config.MonitorsMap()
	for _, g := range config.Graphs {
		graphs[g.Id] = dict{
			"chartCanvas":   "#" + g.Id,
			"chartDelay":    g.ChartDelay,
			"chartOptions":  g.ChartOptions,
			"seriesOptions": g.SeriesOptions,
			"timeOptions":   g.TimeOptions,
			"legendOptions": dict{
				"selector": "#" + g.Id + "_legend",
				"title":    monitors[g.Id].Title,
			},
		}
	}
	return dict{
		"url":     "/metrics",
		"timeout": 1000,
		"graphs":  graphs,
		"controls": dict{
			"startButton": "#start_btn",
			"resetButton": "#reset_btn",
		},
	}
}
