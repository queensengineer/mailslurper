package mailslurper

import (
	"testing"

	"github.com/adampresley/sanitizer"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSMTPMailItem(t *testing.T) {
	Convey("NewSMTPMailItem returns a SMTPMailItem structure", t, func() {
		emailValidationService := NewEmailValidationService()
		xssService := sanitizer.NewXSSService()

		expected := &SMTPMailItem{
			FromAddress: "",
			ToAddresses: make([]string, 0),

			EmailValidationService: emailValidationService,
			XSSService:             xssService,
		}

		actual := NewSMTPMailItem(emailValidationService, xssService)
		So(actual, ShouldResemble, expected)
	})

	Convey("ProcessBody", t, func() {
		emailValidationService := NewEmailValidationService()
		xssService := sanitizer.NewXSSService()
		smtpMailItem := NewSMTPMailItem(emailValidationService, xssService)

		streamInput := "Content-Type: multipart/related;\r\n boundary=\"===============4727162196409731038==\"\r\nMIME-Version: 1.0\r\nFrom: mail@example.com\r\nTo: mail@example.com\r\nSubject: Test...\r\nDate: Tue, 19 Apr 2016 23:32:02 -0500\r\n\r\nThis is a multi-part message in MIME format.\r\n--===============4727162196409731038==\r\nContent-Type: multipart/alternative; boundary=\"===============5399255960384274459==\"\r\nMIME-Version: 1.0\r\n\r\n--===============5399255960384274459==\r\nContent-Type: text/plain; charset=\"us-ascii\"\r\nMIME-Version: 1.0\r\nContent-Transfer-Encoding: 7bit\r\n\r\nTest message... please ignore.\r\n--===============5399255960384274459==\r\nContent-Type: text/html; charset=\"us-ascii\"\r\nMIME-Version: 1.0\r\nContent-Transfer-Encoding: 7bit\r\n\r\n<p>Test message... please ignore.</p>\r\n--===============5399255960384274459==--\r\n\r\n--===============4727162196409731038==--"

		smtpMailItem.ProcessBody(streamInput)
	})

	Convey("ProcessFrom", t, func() {
		emailValidationService := NewEmailValidationService()
		xssService := sanitizer.NewXSSService()
		smtpMailItem := NewSMTPMailItem(emailValidationService, xssService)

		Convey("returns an InvalidCommandError when the stream input contains an invalid command", func() {
			expected := InvalidCommand("MAIL FROM")
			actual := smtpMailItem.ProcessFrom("MAIL: from@test.com")
			So(actual, ShouldResemble, expected)
		})

		Convey("returns an InvalidCommandFormatError when the stream input has the correct command, but is missing a value", func() {
			expected := InvalidCommandFormat("MAIL FROM")
			actual := smtpMailItem.ProcessFrom("MAIL FROM from@test.com")
			So(actual, ShouldResemble, expected)
		})

		Convey("returns an InvalidEmailError when the from address is not a valid email address", func() {
			expected := InvalidEmail("from@")
			actual := smtpMailItem.ProcessFrom("MAIL FROM: from@\n")
			So(actual, ShouldResemble, expected)
		})

		Convey("returns nil and stores the email address when all is valid", func() {
			expectedAddress := "from@test.com"
			actual := smtpMailItem.ProcessFrom("MAIL FROM: from@test.com\n")

			So(actual, ShouldBeNil)
			So(smtpMailItem.FromAddress, ShouldEqual, expectedAddress)
		})
	})

	Convey("ProcessRecipient", t, func() {
		emailValidationService := NewEmailValidationService()
		xssService := sanitizer.NewXSSService()
		smtpMailItem := NewSMTPMailItem(emailValidationService, xssService)

		Convey("returns an InvalidCommandError when the stream input contains an invalid command", func() {
			expected := InvalidCommand("RCPT TO")
			actual := smtpMailItem.ProcessRecipient("TO: from@test.com")
			So(actual, ShouldResemble, expected)
		})

		Convey("returns an InvalidCommandFormatError when the stream input has the correct command, but is missing a value", func() {
			expected := InvalidCommandFormat("RCPT TO")
			actual := smtpMailItem.ProcessRecipient("RCPT TO from@test.com")
			So(actual, ShouldResemble, expected)
		})

		Convey("returns an InvalidEmailError when the to address is not a valid email address", func() {
			expected := InvalidEmail("from@")
			actual := smtpMailItem.ProcessRecipient("RCPT TO: from@\n")
			So(actual, ShouldResemble, expected)
		})

		Convey("returns nil and stores the email address when all is valid", func() {
			expectedAddress := []string{"from@test.com"}
			actual := smtpMailItem.ProcessRecipient("RCPT TO: from@test.com\n")

			So(actual, ShouldBeNil)
			So(smtpMailItem.ToAddresses, ShouldResemble, expectedAddress)
		})
	})
}
