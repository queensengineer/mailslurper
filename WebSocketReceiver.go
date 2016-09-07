// Copyright 2013-2014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package mailslurper

import(
	"log"
)

type WebSocketReceiver struct{}

func (this WebSocketReceiver) Receive(mailItem *MailItem) error {
	websocket.BroadcastMessageToWebSockets(*mailItem)

	log.Println("INFO - Mail item", mailItem.Id, "broadcast to websockets")
	return nil
}