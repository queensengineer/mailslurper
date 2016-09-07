package mailslurper

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSet(t *testing.T) {
	Convey("A Set of header items can be unfolded", t, func() {
		set := &Set{}
		headers := "Content-Type: text/html\r\n boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer: This is\r\n a test\r\n"

		expected := "Content-Type: text/html boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer: This is a test\r\n"
		actual := set.UnfoldHeaders(headers)

		So(actual, ShouldEqual, expected)
	})

	Convey("Parsing a set of headers", t, func() {
		Convey("with an invalid header will return an error", func() {
			headers := "Content-Type: text/html\r\n boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer This is\r\n a test\r\n"
			_, err := NewHeaderSet(headers)
			expected := "Error parsing header"

			So(err.Error(), ShouldContainSubstring, expected)
		})

		Convey("creates an array of Item structures", func() {
			headers := "Content-Type: text/html\r\n boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer: This is\r\n a test\r\n"
			actual, _ := NewHeaderSet(headers)
			expected := &Set{
				HeaderItems: []IItem{
					&Item{"Content-Type", []string{"text/html boundary=\"abcd\""}},
					&Item{"Subject", []string{"Test"}},
					&Item{"X-Mailer", []string{"This is a test"}},
				},
			}

			So(actual, ShouldResemble, expected)
		})
	})

	Convey("Getting a header by name", t, func() {
		set, _ := NewHeaderSet("Content-Type: text/html\r\n boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer: This is\r\n a test\r\n")

		Convey("using a valid header name returns the header item", func() {
			expected := &Item{
				Key:    "Subject",
				Values: []string{"Test"},
			}

			match, err := set.Get("subject")
			So(err, ShouldBeNil)

			actual := match.(*Item)
			So(actual, ShouldResemble, expected)
		})

		Convey("using an invalid header name returns an error", func() {
			expected := smtperrors.MissingHeader("bob")
			match, err := set.Get("bob")

			So(match, ShouldBeNil)
			So(err, ShouldResemble, expected)
		})
	})

	Convey("Set can be converted to a map", t, func() {
		set, _ := NewHeaderSet("Content-Type: text/html\r\n boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer: This is\r\n a test\r\n")

		expected := map[string][]string{
			"Content-Type": []string{"text/html boundary=\"abcd\""},
			"Subject":      []string{"Test"},
			"X-Mailer":     []string{"This is a test"},
		}
		actual := set.ToMap()

		So(actual, ShouldResemble, expected)
	})
}
