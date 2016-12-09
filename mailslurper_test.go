package mailslurper_test

import (
	"fmt"
	"net/smtp"

	"github.com/mailslurper/mailslurper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mailslurper", func() {
	var address string
	var auth smtp.Auth

	BeforeEach(func() {
		address = "localhost:2500"
		auth = smtp.PlainAuth("", "", "", "adampresley.com")

		DeleteAllMail()
	})

	Describe("Sending a valid email", func() {
		var from string

		BeforeEach(func() {
			from = "adam@adampresley.com"
		})

		/*
		 * Valid plain text email
		 */
		Context("that is text/plain", func() {
			It("records the plain text in the database", func() {
				var err error
				var mailItems []mailslurper.MailItem

				body := "This is a plain text email"
				to := []string{"bob@test.com"}
				msg := []byte("To: bob@test.com\r\n" +
					"Subject: Plain Text Test\r\n" +
					"Date: Thu, 08 Dec 2016 23:46:05 -0600 CST\r\n" +
					"Content-Type: text/plain\r\n" +
					"\r\n" +
					body + "\r\n")

				smtp.SendMail(address, auth, from, to, msg)

				search := &mailslurper.MailSearch{}
				if mailItems, err = database.GetMailCollection(0, 1, search); err != nil {
					Fail(fmt.Sprintf("Error getting mail collection from database: %s", err.Error()))
				}

				Expect(len(mailItems)).To(Equal(1))
				Expect(mailItems[0].Subject).To(Equal("Plain Text Test"))
				Expect(mailItems[0].DateSent).To(Equal("2016-12-08 23:46:05"))
				Expect(mailItems[0].ContentType).To(Equal("text/plain"))
				Expect(mailItems[0].FromAddress).To(Equal(from))
				Expect(mailItems[0].ToAddresses).To(Equal([]string{
					"bob@test.com",
				}))
				Expect(mailItems[0].Body).To(Equal(body))
			})
		})
	})
})
