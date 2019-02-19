package api

import (
	"fmt"
	"log"
	"strings"

	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

// Format values
const (
	// Default
	messageFormatFull = "full"
	// Labels only
	messageFormatMinimal = "minimal"
	// Labels and payload data
	messageFormatMetadata = "metadata"
	messageFormatRaw      = "raw"
)

type AccountHelper struct {
	User string

	srv *gm.Service
}

func NewAccountHelper(user string, srv *gm.Service) *AccountHelper {
	return &AccountHelper{User: user, srv: srv}
}

func (h *AccountHelper) GetEmailAddress() (string, error) {
	r, err := h.srv.Users.GetProfile(h.User).Do()
	if err != nil {
		return "", err
	}
	return r.EmailAddress, nil
}

type MsgHelper struct {
	User string

	srv    *gm.Service
	labels map[string]string // Label ID to label name
}

func NewMsgHelper(user string, srv *gm.Service) *MsgHelper {
	return &MsgHelper{User: user, srv: srv}
}

// ---------- Message methods ----------------

func (h *MsgHelper) loadLabels() error {
	util.Debugln("Loading labels")
	r, err := h.srv.Users.Labels.List(h.User).Do()
	if err != nil {
		return err
	}
	labelMap := make(map[string]string)
	for _, l := range r.Labels {
		labelMap[l.Id] = l.Name
	}
	h.labels = labelMap
	return nil
}

func (h *MsgHelper) requireLabels() {
	if h.labels == nil {
		err := h.loadLabels()
		if err != nil {
			log.Fatalf("Failed to load labels: %v\n", err)
		}
	}
}

func (h *MsgHelper) LabelName(lblId string) string {
	h.requireLabels()
	return h.labels[lblId]
}

func (h *MsgHelper) LabelNames(ids []string) []string {
	h.requireLabels()
	var lNames []string
	for _, lId := range ids {
		lNames = append(lNames, h.labels[lId])
	}
	return lNames
}

func (h *MsgHelper) MessageLabelNames(m *gm.Message) []string {
	return h.LabelNames(m.LabelIds)
}

func (h *MsgHelper) LoadDetailedMessages(msgs []*gm.Message) (
	[]*gm.Message, error) {

	format := messageFormatMetadata

	var detailedMsgs []*gm.Message

	printLvl := prnt.Always
	prnt.HPrint(printLvl, "Loading message details ")
	for i, msg := range msgs {
		progressStr := fmt.Sprintf("%d/%d", i+1, len(msgs))
		prnt.HPrint(printLvl, progressStr)

		dMsg, err := h.srv.Users.Messages.Get(h.User, msg.Id).Format(format).Do()
		if err != nil {
			return nil, fmt.Errorf("Failed to get message: %v", err)
		}
		detailedMsgs = append(detailedMsgs, dMsg)
		prnt.HPrint(printLvl, strings.Repeat("\x08", len(progressStr)))
	}
	prnt.HPrint(printLvl, "\n")

	return detailedMsgs, nil
}

func (h *MsgHelper) LoadDetailedUncachedMessages(msgs []*gm.Message, cache *Cache) (
	[]*gm.Message, error) {

	newMsgs := make([]*gm.Message, 0, len(msgs))
	cachedMsgs := make([]*gm.Message, 0, len(msgs))
	for _, msg := range msgs {
		if cMsg, ok := cache.Msgs[msg.Id]; ok {
			cachedMsgs = append(cachedMsgs, cMsg)
		} else {
			newMsgs = append(newMsgs, msg)
		}
	}
	var err error
	newMsgs, err = h.LoadDetailedMessages(newMsgs)
	if err != nil {
		return nil, err
	}
	return append(cachedMsgs, newMsgs...), nil
}

func (h *MsgHelper) LoadMessage(id string) (*gm.Message, error) {
	return h.srv.Users.Messages.Get(h.User, id).Do()
}

type MessageDetailLevel int

const (
	IdsOnly MessageDetailLevel = iota
	LabelsOnly
	LabelsAndPayload
)

/* Query the server for messages.
 * maxMsgs: a value greater than 0 to apply a max
 */
func (h *MsgHelper) QueryMessages(
	query string, inboxOnly bool, unreadOnly bool, maxMsgs int64,
	detailLevel MessageDetailLevel) ([]*gm.Message, error) {

	fullQuery := ""
	if inboxOnly {
		fullQuery += "in:inbox "
	}
	if unreadOnly {
		fullQuery += "label:unread "
	}

	fullQuery += query

	pageToken := ""
	queriedPageCnt := 0
	var msgs []*gm.Message

	for queriedPageCnt == 0 || pageToken != "" {
		queriedPageCnt++
		util.Debugf("Querying messages: '%s', page: %d\n", fullQuery, queriedPageCnt)

		call := h.srv.Users.Messages.List(h.User).Q(fullQuery)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		if maxMsgs > 0 {
			call = call.MaxResults(maxMsgs)
		}
		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("Unable to get messages: %v", err)
		}

		pageToken = r.NextPageToken

		for _, m := range r.Messages {
			msgs = append(msgs, m)
			if maxMsgs > 0 && int64(len(msgs)) == maxMsgs {
				// Signal no more pages that we want to read.
				pageToken = ""
				break
			}
		}
	}

	if detailLevel != IdsOnly {
		var err error
		msgs, err = h.LoadDetailedMessages(msgs)
		if err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

func (h *MsgHelper) LabelIdFromName(label string) string {
	h.requireLabels()
	for lId, lName := range h.labels {
		if label == lName {
			return lId
		}
	}
	log.Fatalf("No label named %s found\n", label)
	return ""
}

const (
	MaxBatchModifySize = 500
)

func (h *MsgHelper) BatchModifyMessages(msgs []*gm.Message,
	modReq *gm.BatchModifyMessagesRequest) error {

	var err error
	nMsgs := len(msgs)
	msgsLeft := nMsgs

	nBatches := nMsgs / MaxBatchModifySize
	if nMsgs%MaxBatchModifySize != 0 {
		nBatches++
	}

	prnt.HPrintf(prnt.Quietable, "Applying changes to ")
	msgIdx := 0
	for batch := 0; batch < nBatches; batch++ {
		batchSize := util.IntMin(msgsLeft, MaxBatchModifySize)
		prnt.HPrintf(prnt.Quietable, "%d... ", msgIdx+batchSize)

		msgIds := make([]string, 0, batchSize)
		for i := 0; i < batchSize; i++ {
			msgIds = append(msgIds, msgs[msgIdx].Id)
			msgIdx++
		}

		modReq.Ids = msgIds
		err = h.srv.Users.Messages.BatchModify(h.User, modReq).Do()
		if err != nil {
			return err
		}

		msgsLeft -= batchSize
	}
	prnt.HPrintf(prnt.Quietable, "Done\n")

	return nil
}

func (h *MsgHelper) ApplyLabels(msgs []*gm.Message, labelNames []string) error {
	labelIds := make([]string, 0, len(labelNames))
	for _, labelName := range labelNames {
		labelIds = append(labelIds, h.LabelIdFromName(labelName))
	}

	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds: labelIds,
	}
	return h.BatchModifyMessages(msgs, &modReq)
}
