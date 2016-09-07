package mailslurper

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGenerateID(t *testing.T) {
	Convey("GenerateID returns a UUIID", t, func() {
		expectedLength := 36
		result, _ := GenerateId()

		So(len(result), ShouldEqual, expectedLength)
	})
}
