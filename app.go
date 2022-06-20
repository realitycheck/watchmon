package watchmon

import (
	"net/http"
	"time"
)

type Application struct {
	ws *WatchService
	hs *HTTPService
}

func NewApplication(config AppConfig) *Application {
	app := &Application{}
	initWatchService(app, config)
	initHTTPService(app, config)
	return app
}

func (app *Application) Start(delay time.Duration) {
	app.ws.Start(delay)
}

func (app *Application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.hs.mux.ServeHTTP(w, r)
}
