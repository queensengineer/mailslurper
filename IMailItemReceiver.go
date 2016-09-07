// Copyright 2013-2014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

type IMailItemReceiver interface{
	Receive(mailItem *MailItem) error
}
