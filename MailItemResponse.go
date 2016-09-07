// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

/*
A MailItemResponse is sent in response to a request for a single
mail item
*/
type MailItemResponse struct {
	MailItem MailItem `json:"mailItem"`
}
