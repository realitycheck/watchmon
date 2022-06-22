package app

import log "github.com/sirupsen/logrus"

func newLogger(module string) func(logger string) *log.Entry {
	return func(logger string) *log.Entry {
		return log.WithFields(log.Fields{
			"Namespace": module,
			"Name":      logger,
		})
	}
}

var (
	configLog = newLogger("config")
	httpLog   = newLogger("http")
	watchLog  = newLogger("watch")
)
