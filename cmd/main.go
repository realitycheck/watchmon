package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/realitycheck/watchmon"
)

var (
	addr  = "127.0.0.1:8081"
	delay = 1 * time.Second

	config = ""
	debug  = false
	quiet  = false
)

func init() {
	flag.StringVar(&addr, "addr", addr, "Server address")

	flag.DurationVar(&delay, "d", delay, "Delay of source reading")

	flag.StringVar(&config, "f", config, "Config file")
	flag.BoolVar(&debug, "debug", debug, "Debug mode, enable for verbose logging (default false)")
	flag.BoolVar(&quiet, "quiet", quiet, "Quiet mode, enable to log nothing (default false)")
}

func main() {
	flag.Parse()

	if quiet {
		log.SetOutput(io.Discard)
	}
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	app := watchmon.NewApplication(watchmon.MustLoadConfig(config))

	go app.Start(delay)

	fmt.Printf("Start watchmon at http://%s", addr)

	http.ListenAndServe(addr, app)
}
