package api

import (
	gm "google.golang.org/api/gmail/v1"
)

// A minified version of gm.Thread,  which does not store the messages themselves,
// but rather their Ids.
// TODO this is probably not actually necessary
type Thread struct {
	Id     string
	MsgIds []string
}

func ThreadFromGmailThread(t *gm.Thread) *Thread {
	thread := &Thread{
		Id:     t.Id,
		MsgIds: make([]string, 0, len(t.Messages)),
	}
	for _, msg := range t.Messages {
		thread.MsgIds = append(thread.MsgIds, msg.Id)
	}
	return thread
}
