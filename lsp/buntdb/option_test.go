package buntdb

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestOption(t *testing.T) {
	var o *option
	o.getInnerExpire()
	o.getNoOverWrite()
	o.getExpire()
	o.getIgnoreNotFound()
	o.getIgnoreExpire()

	o = getOption(SetExpireOpt(time.Hour))
	assert.EqualValues(t, time.Hour, o.getExpire())
}
