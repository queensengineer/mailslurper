package main

import (
	"net/http"

	"github.com/adampresley/webframework/httpService"
	"github.com/mailslurper/mailslurper"
)

/*
admin is the page for performing administrative tasks in MailSlurper
*/
func admin(writer http.ResponseWriter, request *http.Request) {
	var err error

	data := mailslurper.Page{
		Title: "Admin",
	}

	if err = renderMainLayout(writer, request, "admin.html", data); err != nil {
		httpService.WriteText(writer, err.Error(), 500)
	}
}

/*
index is the main view. This endpoint provides the email list and email detail
views.
*/
func index(writer http.ResponseWriter, request *http.Request) {
	var err error

	data := mailslurper.Page{
		Title: "Mail",
	}

	if err = renderMainLayout(writer, request, "index.html", data); err != nil {
		httpService.WriteText(writer, err.Error(), 500)
	}
}

/*
manageSavedSearches is the page for managing saved searches
*/
func manageSavedSearches(writer http.ResponseWriter, request *http.Request) {
	var err error

	data := mailslurper.Page{
		Title: "Manage Saved Searches",
	}

	if err = renderMainLayout(writer, request, "manageSavedSearches.html", data); err != nil {
		httpService.WriteText(writer, err.Error(), 500)
	}
}

/*
getPruneOptions returns a set of valid pruning options.

	GET: /v1/pruneoptions
*/
func getPruneOptions(writer http.ResponseWriter, request *http.Request) {
	httpService.WriteJSON(writer, mailslurper.PruneOptions, 200)
}
