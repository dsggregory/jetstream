package jetstream

import (
	"testing"

	"github.com/nats-io/nats.go"

	. "github.com/smartystreets/goconvey/convey"
)

func TestExpandURLs(t *testing.T) {

	Convey("expand URLs", t, func() {
		rslt := expandClusterURL(nats.DefaultURL)
		So(rslt, ShouldEqual, nats.DefaultURL)

		href := "http://mail.yahoo.com" // has multiple A records
		rslt = expandClusterURL(href)
		So(rslt, ShouldNotEqual, href)
	})
	Convey("use default", t, func() {
		href := "http://localhost:4222,http://localhost2:4222"
		rslt := expandClusterURL(href)
		So(rslt, ShouldEqual, href)

		href = "http://_notvalid_:4222"
		rslt = expandClusterURL(href)
		So(rslt, ShouldEqual, href)
	})
}
