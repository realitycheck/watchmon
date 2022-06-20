package watchmon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_makeConfigData(t *testing.T) {
	d := makeConfigData(testConfig)

	want := `{
		"url": "/metrics",
		"timeout": 1000,
		"controls": {
			"resetButton": "#reset_btn",
			"startButton": "#start_btn"
		},
		"graphs": {
			"arris_downstream_power": {
				"chartDelay": 1000,
				"chartCanvas": "#arris_downstream_power",
				"chartOptions": {
					"interpolation": "step"
				},
				"legendOptions": {
					"selector": "#arris_downstream_power_legend",
					"title": "Downstream Frequency"
				},
				"seriesOptions": {},
				"timeOptions": {}
			}
		}
	}`

	got, err := json.Marshal(d)
	assert.NoError(t, err)
	assert.JSONEq(t, string(got), want)
}

func Test_makeTemplatesData(t *testing.T) {
	d := makeTemplatesData(testConfig)

	want := `{
		"index.html": {
			"Canvas": [
				{
					"Title": "Downstream",
					"Monitors": [
						{
							"Id": "arris_downstream_power",
							"Title": "Downstream Frequency",
							"Type": "gauge",
							"Value": {
								"SourceId": "arris",
								"RecordId": "downstream",
								"Format": "%f dBmV",
								"Header": "power",
								"Labels": [{
									"Format": "",
									"Header": "dcid"
								}, {
									"Format": "",
									"Header": "name"
								}]
							}
						},
						{
							"Id": "arris_downstream_snr",
							"Title": "Downstream SNR",
							"Type": "gauge",
							"Value": {
								"SourceId": "arris",
								"RecordId": "downstream",
								"Format": "%f dB",
								"Header": "snr",
								"Labels": [{
									"Format": "",
									"Header": "dcid"
								}, {
									"Format": "",
									"Header": "name"
								}]
							}
						}
					]
				}
			]
		}
	}`

	got, err := json.Marshal(d)
	assert.NoError(t, err)
	assert.JSONEq(t, string(got), want)
}

func Test_HTTPService_serve(t *testing.T) {
	tests := []struct {
		name       string
		h          http.HandlerFunc
		req        *http.Request
		wantStatus int
	}{
		{
			"serveConfigData: ok",
			(&HTTPService{
				configData: makeConfigData(testConfig),
			}).serveConfigData,
			httptest.NewRequest("GET", "http://example.com/config.json", nil),
			200,
		},
		{
			"serveConfigData: error",
			(&HTTPService{
				configData: dict{
					"encode error": func() {},
				},
			}).serveConfigData,
			httptest.NewRequest("GET", "http://example.com/config.json", nil),
			500,
		},
		{
			"serveRoot: ok",
			(&HTTPService{
				templatesData: makeTemplatesData(testConfig),
			}).serveRoot,
			httptest.NewRequest("GET", "http://example.com/", nil),
			200,
		},
		{
			"serveRoot: ok (index.html)",
			(&HTTPService{
				templatesData: makeTemplatesData(testConfig),
			}).serveRoot,
			httptest.NewRequest("GET", "http://example.com/index.html", nil),
			200,
		},
		{
			"serveRoot: error 404",
			(&HTTPService{
				templatesData: makeTemplatesData(testConfig),
			}).serveRoot,
			httptest.NewRequest("GET", "http://example.com/not_found.html", nil),
			404,
		},
		{
			"serveRoot: error 500",
			(&HTTPService{
				templatesData: map[string]dict{
					"index.html": {
						"Canvas": 123,
					},
				},
			}).serveRoot,
			httptest.NewRequest("GET", "http://example.com/", nil),
			200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.h(w, tt.req)

			r := w.Result()
			assert.Equal(t, tt.wantStatus, r.StatusCode)
		})
	}

}
