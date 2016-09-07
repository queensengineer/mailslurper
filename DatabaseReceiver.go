// Copyright 2013-2014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"log"
)

type DatabaseReceiver struct {
	database storage.IStorage
}

func NewDatabaseReceiver(database storage.IStorage) DatabaseReceiver {
	return DatabaseReceiver{
		database: database,
	}
}

func (this DatabaseReceiver) Receive(mailItem *MailItem) error {
	var err error
	var newID string

	if newID, err = this.database.StoreMail(mailItem); err != nil {
		log.Println("libmailslurper: ERROR - There was an error while storing your mail item:", err)
		return err
	}

	log.Println("libmailslurper: INFO - Mail item", newID, "written")
	return nil
}
