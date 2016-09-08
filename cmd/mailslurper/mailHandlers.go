package main

import (
	"bytes"
	"encoding/base64"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adampresley/webframework/httpService"
	"github.com/mailslurper/libmailslurper/model/attachment"
	"github.com/mailslurper/mailslurper"
)

func mailEndpoint(writer http.ResponseWriter, request *http.Request) {
	pathParts := splitPath(request)

	switch strings.ToLower(request.Method) {
	case "get":
		if len(pathParts) == 2 {
			getMail(writer, request)
		}

		if len(pathParts) == 3 && pathParts[2] == "message" {
			getMailMessage(writer, request)
		}

		if len(pathParts) == 4 && pathParts[2] == "attachment" {
			downloadAttachment(writer, request)
		}

	case "delete":
		deleteMail(writer, request)

	default:
		httpService.WriteText(writer, "Not default", 404)
	}
}

/*
deleteMail is a request to delete mail items. This expects a body containing
a DeleteMailRequest object.

	DELETE: /mail/{pruneCode}
*/
func deleteMail(writer http.ResponseWriter, request *http.Request) {
	var err error

	if !isVerb(request, "DELETE") {
		httpService.WriteText(writer, "Not found", 404)
		return
	}

	pathParts := parsePath(request, "/mail/{pruneCode}")

	if len(pathParts) < 2 {
		log.Errorf("Invalid delete mail request")
		httpService.WriteText(writer, "Invalid delete mail request", 400)
		return
	}

	pruneCode := mailslurper.PruneCode(pathParts["pruneCode"])

	if !pruneCode.IsValid() {
		log.Errorf("Attempt to use invalid prune code - %s", pruneCode)
		httpService.WriteText(writer, "Invalid prune type", 400)
		return
	}

	startDate := pruneCode.ConvertToDate()

	if err = database.DeleteMailsAfterDate(startDate); err != nil {
		log.Errorf("Problem deleting mails - %s", err.Error())
		httpService.WriteText(writer, "There was a problem deleting mails", 500)
		return
	}

	log.Infof("Deleting mails, code %s - Start - %s", pruneCode.String(), startDate)
	httpService.WriteText(writer, "OK", 200)
}

/*
getMail returns a single mail item by ID.

	GET: /mail/{id}
*/
func getMail(writer http.ResponseWriter, request *http.Request) {
	var mailID string
	var mailItem MailItem
	var err error
	var ok bool

	if !isVerb(request, "GET") {
		httpService.WriteText(writer, "Not found", 404)
		return
	}

	pathParts := parsePath(request, "/mail/{id}")

	/*
	 * Validate incoming arguments
	 */
	if mailID, ok = pathParts["mailID"]; !ok {
		log.Error("Invalid mail ID passed to GetMail")
		httpService.WriteText(writer, "A valid mail ID is required", 400)
		return
	}

	/*
	 * Retrieve the mail item
	 */
	if mailItem, err = database.GetMailByID(mailID); err != nil {
		log.Errorf("Problem getting mail item in GetMail - %s", err.Error())
		httpService.WriteText(writer, "Problem getting mail item", 500)
		return
	}

	log.Infof("Mail item %s retrieved", mailID)

	result := &mailslurper.MailItemResponse{
		MailItem: mailItem,
	}

	httpService.WriteJSON(writer, result, 200)
}

/*
getMailCollection returns a collection of mail items. This is constrianed
by a page number. A page of data contains 50 items.

	GET: /mails?pageNumber={pageNumber}
*/
func getMailCollection(writer http.ResponseWriter, request *http.Request) {
	var err error
	var pageNumberString string
	var pageNumber int
	var mailCollection []MailItem
	var totalRecordCount int

	/*
	 * Validate incoming arguments. A page is currently 50 items, hard coded
	 */
	pageNumberString = request.URL.Query().Get("pageNumber")
	if pageNumberString == "" {
		pageNumber = 1
	} else {
		if pageNumber, err = strconv.Atoi(pageNumberString); err != nil {
			log.Error("Invalid page number passed to GetMailCollection")
			httpService.WriteText(writer, "A valid page number is required", 400)
			return
		}
	}

	length := 50
	offset := (pageNumber - 1) * length

	/*
	 * Retrieve mail items
	 */
	mailSearch := &MailSearch{
		Message: request.URL.Query().Get("message"),
		Start:   request.URL.Query().Get("start"),
		End:     request.URL.Query().Get("end"),
		From:    request.URL.Query().Get("from"),
		To:      request.URL.Query().Get("to"),

		OrderByField:     request.URL.Query().Get("orderby"),
		OrderByDirection: request.URL.Query().Get("dir"),
	}

	if mailCollection, err = database.GetMailCollection(offset, length, mailSearch); err != nil {
		log.Errorf("Problem getting mail collection - %s", err.Error())
		httpService.WriteText(writer, "Problem getting mail collection", 500)
		return
	}

	if totalRecordCount, err = database.GetMailCount(mailSearch); err != nil {
		log.Errorf("Problem getting record count in GetMailCollection - %s", err.Error())
		httpService.WriteText(writer, "Error getting record count", 500)
		return
	}

	totalPages := int(math.Ceil(float64(totalRecordCount / length)))
	if totalPages*length < totalRecordCount {
		totalPages++
	}

	log.Infof("Mail collection page %d retrieved", pageNumber)

	result := &mailslurper.MailCollectionResponse{
		MailItems:    mailCollection,
		TotalPages:   totalPages,
		TotalRecords: totalRecordCount,
	}

	httpService.WriteJSON(writer, result, 200)
}

/*
getMailCount returns the number of mail items in storage.

	GET: /mailcount
*/
func getMailCount(writer http.ResponseWriter, request *http.Request) {
	var err error
	var mailItemCount int

	/*
	 * Get the count
	 */
	if mailItemCount, err = database.GetMailCount(&MailSearch{}); err != nil {
		log.Errorf("Problem getting mail item count in GetMailCount - %s", err.Error())
		httpService.WriteText(writer, "Problem getting mail count", 500)
		return
	}

	log.Infof("Mail item count - %d", mailItemCount)

	result := mailslurper.MailCountResponse{
		MailCount: mailItemCount,
	}

	httpService.WriteJSON(writer, result, 200)
}

/*
getMailMessage returns the message contents of a single mail item

	GET: /mail/{id}/message
*/
func getMailMessage(writer http.ResponseWriter, request *http.Request) {
	var mailID string
	var mailItem MailItem
	var err error
	var ok bool

	if !isVerb(request, "GET") {
		httpService.WriteText(writer, "Not found", 404)
		return
	}

	pathParts := parsePath(request, "/mail/{id}/message")

	if len(pathParts) < 3 {
		httpService.WriteText(writer, "Bad request", 400)
		return
	}

	/*
	 * Validate incoming arguments
	 */
	if mailID, ok = pathParts["mailID"]; !ok {
		log.Error("Invalid mail ID passed to GetMailMessage")
		httpService.WriteText(writer, "A valid mail ID is required", 400)
		return
	}

	/*
	 * Retrieve the mail item
	 */
	if mailItem, err = database.GetMailByID(mailID); err != nil {
		log.Errorf("Problem getting mail item in GetMailMessage - %s", err.Error())
		httpService.WriteText(writer, "Problem getting mail item", 500)
		return
	}

	log.Infof("Mail item %s retrieved", mailID)
	httpService.WriteHTML(writer, mailItem.Body, 200)
}

/*
downloadAttachment retrieves binary database from storage and streams
it back to the caller
*/
func downloadAttachment(writer http.ResponseWriter, request *http.Request) {
	var err error
	var attachmentID string
	var mailID string
	var ok bool

	var attachment attachment.Attachment
	var data []byte

	if !isVerb(request, "GET") {
		httpService.WriteText(writer, "Not found", 404)
		return
	}

	pathParts := parsePath(request, "/mail/{mailID}/attachment/{attachmentID}")

	if len(pathParts) < 4 {
		httpService.WriteText(writer, "Not found", 404)
		return
	}

	/*
	 * Validate incoming arguments
	 */
	if mailID, ok = pathParts["mailID"]; !ok {
		log.Error("No valid mail ID passed to DownloadAttachment")
		httpService.WriteText(writer, "A valid mail ID is required", 400)
		return
	}

	if attachmentID, ok = pathParts["attachmentID"]; !ok {
		log.Error("No valid attachment ID passed to DownloadAttachment")
		httpService.WriteText(writer, "A valid attachment ID is required", 400)
		return
	}

	/*
	 * Retrieve the attachment
	 */
	if attachment, err = database.GetAttachment(mailID, attachmentID); err != nil {
		log.Errorf("Problem getting attachment %s - %s", attachmentID, err.Error())
		httpService.WriteText(writer, "Error getting attachment", 500)
		return
	}

	/*
	 * Decode the base64 data and stream it back
	 */
	if attachment.IsContentBase64() {
		data, err = base64.StdEncoding.DecodeString(attachment.Contents)
		if err != nil {
			log.Errorf("Problem decoding attachment %s - %s", attachmentID, err.Error())
			httpService.WriteText(writer, "Cannot decode attachment", 500)
			return
		}
	} else {
		data = []byte(attachment.Contents)
	}

	log.Infof("Attachment %s retrieved", attachmentID)

	reader := bytes.NewReader(data)
	http.ServeContent(writer, request, attachment.Headers.FileName, time.Now(), reader)
}
