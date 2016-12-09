// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import (
	"sync"

	"github.com/adampresley/webframework/logging2"
)

/*
A DatabaseReceiver takes a MailItem and writes it to a database
*/
type DatabaseReceiver struct {
	database IStorage
	logger   logging2.ILogger
}

/*
NewDatabaseReceiver creates a new DatabaseReceiver object
*/
func NewDatabaseReceiver(database IStorage, logger logging2.ILogger) DatabaseReceiver {
	return DatabaseReceiver{
		database: database,
		logger:   logger,
	}
}

/*
Receive takes a MailItem and writes it to the provided storage engine
*/
func (receiver DatabaseReceiver) Receive(mailItem *MailItem, wg *sync.WaitGroup) error {
	var err error
	var newID string

	wg.Add(1)

	if newID, err = receiver.database.StoreMail(mailItem); err != nil {
		receiver.logger.Errorf("There was an error while storing your mail item: %s", err.Error())
		return err
	}

	receiver.logger.Infof("Mail item %s written", newID)

	wg.Done()
	return nil
}
