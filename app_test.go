package watchmon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewApplication(t *testing.T) {
	got := NewApplication(testConfig)
	assert.Equal(t, len(got.ws.monitors), len(testConfig.Monitors))
	assert.Equal(t, len(got.ws.sources), len(testConfig.Sources))
}
