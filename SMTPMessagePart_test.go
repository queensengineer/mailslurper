package mailslurper

import (
	"net/mail"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSMTPMessagePart(t *testing.T) {
	Convey("NewSMTPMessagePart returns a SMTPMessagePart structure", t, func() {
		expected := &SMTPMessagePart{
			Message:      &Message{},
			MessageParts: make([]ISMTPMessagePart, 0),
		}

		actual := NewSMTPMessagePart()
		So(actual, ShouldResemble, expected)
	})

	Convey("When working with headers", t, func() {
		messagePart := NewSMTPMessagePart()

		Convey("a header set can be added", func() {
			set, _ := header.NewHeaderSet("Content-Type: text/html;\r\n boundary=\"abcd\"\r\nSubject: Test\r\nX-Mailer: This is\r\n a test\r\n")

			expected := &Message{
				Header: map[string][]string{
					"Content-Type": []string{"text/html; boundary=\"abcd\""},
					"Subject":      []string{"Test"},
					"X-Mailer":     []string{"This is a test"},
				},
			}
			err := messagePart.AddHeaders(set)
			actual := messagePart.Message

			So(err, ShouldBeNil)
			So(actual, ShouldResemble, expected)
		})
	})

	Convey("When building messages", t, func() {
		messagePart := NewSMTPMessagePart()

		Convey("we can parse a multipart/related with several multipart/alternatives", func() {
			body := "Content-Type: multipart/related;\r\n boundary=\"===============4727162196409731038==\"\r\nMIME-Version: 1.0\r\nFrom: mail@example.com\r\nTo: mail@example.com\r\nSubject: Test...\r\nDate: Tue, 19 Apr 2016 23:32:02 -0500\r\n\r\nThis is a multi-part message in MIME format.\r\n--===============4727162196409731038==\r\nContent-Type: multipart/alternative; boundary=\"===============5399255960384274459==\"\r\nMIME-Version: 1.0\r\n\r\n--===============5399255960384274459==\r\nContent-Type: text/plain; charset=\"us-ascii\"\r\nMIME-Version: 1.0\r\nContent-Transfer-Encoding: 7bit\r\n\r\nTest message... please ignore.\r\n--===============5399255960384274459==\r\nContent-Type: text/html; charset=\"us-ascii\"\r\nMIME-Version: 1.0\r\nContent-Transfer-Encoding: 7bit\r\n\r\n<p>Test message... please ignore.</p>\r\n--===============5399255960384274459==--\r\n\r\n--===============4727162196409731038==--"

			err := messagePart.BuildMessages(body)

			So(err, ShouldBeNil)
		})

		Convey("we can parse a simple plain text email", func() {
			body := "Content-Type: text/plain\r\nMIME-Version: 1.0\r\nFrom: mail@example.com\r\nTo: mail@example.com\r\nSubject: Test...\r\nDate: Tue, 19 Apr 2016 23:32:02 -0500\r\n\r\nThis is a simple text email"

			err := messagePart.BuildMessages(body)

			So(err, ShouldBeNil)
		})

		Convey("we can parse a simple HTML email", func() {
			body := "Content-Type: text/html\r\nMIME-Version: 1.0\r\nFrom: mail@example.com\r\nTo: mail@example.com\r\nSubject: Test...\r\nDate: Tue, 19 Apr 2016 23:32:02 -0500\r\n\r\n<p>This is a simple text email</p>"

			err := messagePart.BuildMessages(body)

			So(err, ShouldBeNil)
		})

		Convey("we can parse a multipart/mixed", func() {
			body := "Content-Type: multipart/mixed; boundary=\"abcd\"\r\nMIME-Version: 1.0\r\nFrom: mail@example.com\r\nTo: mail@example.com\r\nSubject: Test...\r\nDate: Tue, 19 Apr 2016 23:32:02 -0500\r\n\r\n--abcd\r\nContent-Type: text/plain\r\n\r\nThis is the text version\r\n--abcd\r\nContent-Type: text/html\r\n\r\n<p>This is HTML</p>\r\n--abcd--"

			err := messagePart.BuildMessages(body)

			So(err, ShouldBeNil)
		})

		Convey("we can parse a multipart/mixed with related and alternative content", func() {
			body := "Content-Type: multipart/mixed; boundary=\"a\"\r\nMIME-Version: 1.0\r\nFrom: mail@example.com\r\nTo: mail@example.com\r\nSubject: Test...\r\nDate: Tue, 19 Apr 2016 23:32:02 -0500\r\n\r\n--a\r\nContent-Type: multipart/related; boundary=\"b\"\r\n\r\n--b\r\nContent-Type: multipart/alternative; boundary=\"c\"\r\n\r\n--c\r\nContent-Type: text/plain\r\n\r\nThis is the text version\r\n--c\r\nContent-Type: text/html\r\n\r\n<p>This is HTML</p>\r\n--c--\r\n\r\n--b\r\nContent-Type: image/jpeg;name=\"logo.jpg\"\r\nContent-Transfer-Encoding: base64\r\nContent-ID: <logo.png>\r\n\r\nabcdlkjfldkjflskdjfsl=\r\n\r\n--b--\r\n\r\n--a\r\nContent-Type: application/pdf;name=\"file.pdf\"\r\nContent-Transfer-Encoding: base64\r\nContent-Disposition: attachment;filename=\"file.pdf\"\r\n\r\nabcdlkjfsdlkfj=\r\n\r\n--a--"

			err := messagePart.BuildMessages(body)

			So(err, ShouldBeNil)
		})
	})
}
