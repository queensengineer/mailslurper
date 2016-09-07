package mailslurper

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestItem(t *testing.T) {
	Convey("A header Item", t, func() {
		item := &Item{
			Key: "key",
			Values: []string{
				"value",
			},
		}

		Convey("can get its key", func() {
			expected := "key"
			actual := item.GetKey()

			So(actual, ShouldEqual, expected)
		})

		Convey("can get its value", func() {
			expected := []string{"value"}
			actual := item.GetValues()

			So(actual, ShouldResemble, expected)
		})
	})

	Convey("Parsing a single header item", t, func() {
		item := &Item{}

		Convey("returns an error when the header is invalid", func() {
			header := "Subject test"
			expected := InvalidHeader(header)
			actual := item.ParseHeaderString(header)

			So(actual, ShouldResemble, expected)
		})

		Convey("sets the key and values with the parsed content", func() {
			header := "Subject: Test"
			expectedKey := "Subject"
			expectedValue := []string{"Test"}

			err := item.ParseHeaderString(header)

			So(err, ShouldBeNil)
			So(item.GetKey(), ShouldEqual, expectedKey)
			So(item.GetValues(), ShouldResemble, expectedValue)
		})

		Convey("trims space around the key", func() {
			header := "Subject : Test"
			expectedKey := "Subject"

			err := item.ParseHeaderString(header)

			So(err, ShouldBeNil)
			So(item.GetKey(), ShouldEqual, expectedKey)
		})

		Convey("trims space around the value", func() {
			header := "Subject: Test   \t"
			expectedValue := []string{"Test"}

			err := item.ParseHeaderString(header)

			So(err, ShouldBeNil)
			So(item.GetValues(), ShouldResemble, expectedValue)
		})
	})
}
