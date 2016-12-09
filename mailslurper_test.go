package mailslurper_test

import (
	"net/smtp"

	//	"github.com/mailslurper/mailslurper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mailslurper", func() {
	var address string
	var auth smtp.Auth

	BeforeEach(func() {
		address = "localhost:2500"
		auth = smtp.PlainAuth("", "", "", "adampresley.com")
	})

	Describe("Sending a valid email", func() {
		var from string

		BeforeEach(func() {
			from = "adam@adampresley.com"
		})

		Context("that is text/plain", func() {
			It("records the plain text in the database", func() {
				to := []string{"bob@test.com"}
				msg := []byte("To: bob@test.com\r\n" +
					"Subject: Plain Text Test\r\n" +
					"Date: Thu, 08 Dec 2016 23:46:05 -0600 CST\r\n" +
					"Content-Type: text/plain\r\n" +
					"\r\n" +
					"This is a plain text email.\r\n")

				smtp.SendMail(address, auth, from, to, msg)
				Expect(1).To(Equal(1))
			})
		})
	})
})
