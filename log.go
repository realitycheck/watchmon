package watchmon

import log "github.com/sirupsen/logrus"

func newLogger(service string) func(logger string) *log.Entry {
	return func(logger string) *log.Entry {
		return log.WithFields(log.Fields{
			"Service": service,
			"Logger":  logger,
		})
	}
}

var (
	httpLog  = newLogger("http")
	watchLog = newLogger("watch")
)
