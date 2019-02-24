package api

import (
	"fmt"
	"log"

	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

type MessageFormat string

// Format values
const (
	// Default. Provides all parts of the Payload
	messageFormatFull MessageFormat = "full"
	// Labels only
	messageFormatMinimal MessageFormat = "minimal"
	// Labels and payload data
	messageFormatMetadata MessageFormat = "metadata"
	// Only the Raw. Payload will be nil
	MessageFormatRaw MessageFormat = "raw"
)

func (f MessageFormat) ToString() string {
	return string(f)
}

type MessageDetailLevel int

const (
	IdsOnly MessageDetailLevel = iota
	LabelsOnly
	LabelsAndPayload
)

func (dl MessageDetailLevel) Format() MessageFormat {
	var format MessageFormat
	switch dl {
	case IdsOnly:
		format = messageFormatMinimal
	case LabelsOnly:
		format = messageFormatMetadata
	case LabelsAndPayload:
		format = messageFormatFull
	}
	return format
}

func MoreDetailedLevel(a, b MessageDetailLevel) MessageDetailLevel {
	if a > b {
		return a
	}
	return b
}

func MessageMeetsDetailLevel(msg *gm.Message, detail MessageDetailLevel) bool {
	switch detail {
	case IdsOnly:
		return true
	case LabelsOnly:
		return msg.LabelIds != nil && len(msg.LabelIds) > 0
	case LabelsAndPayload:
	}
	return MessageHasBody(msg)
}

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

	cache *Cache
	// These are not cached, because they can change between queries
	loadedThreads map[string]*gm.Thread
}

func NewMsgHelper(user string, srv *gm.Service) *MsgHelper {
	return &MsgHelper{
		User:          user,
		srv:           srv,
		loadedThreads: make(map[string]*gm.Thread),
	}
}

func (h *MsgHelper) getCache() *Cache {
	if h.cache == nil {
		h.cache = NewCache()
		h.cache.LoadMsgs()
	}
	return h.cache
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

func (h *MsgHelper) fetchMessages(msgs []*gm.Message, format MessageFormat) (
	[]*gm.Message, error) {

	var detailedMsgs []*gm.Message

	prnt.Hum.Always.P("Loading message details ")
	progP := prnt.NewProgressPrinter(len(msgs))
	for _, msg := range msgs {
		progP.Progress(1)

		dMsg, err := h.srv.Users.Messages.Get(h.User, msg.Id).
			Format(format.ToString()).Do()
		if err != nil {
			return nil, fmt.Errorf("Failed to get message: %v", err)
		}
		detailedMsgs = append(detailedMsgs, dMsg)
	}
	prnt.Hum.Always.P("\n")

	return detailedMsgs, nil
}

func (h *MsgHelper) fetchThreads(threads []*gm.Thread, detail MessageDetailLevel,
) ([]*gm.Thread, error) {

	var detailedThreads []*gm.Thread

	prnt.Hum.Always.P("Loading message details by thread")
	progP := prnt.NewProgressPrinter(len(threads))
	for _, t := range threads {
		progP.Progress(1)

		dThread, err := h.GetThread(t.Id, detail)
		if err != nil {
			return nil, fmt.Errorf("Failed to get thread: %v", err)
		}
		detailedThreads = append(detailedThreads, dThread)
	}
	prnt.Hum.Always.P("\n")

	return detailedThreads, nil
}

func (h *MsgHelper) LoadMessages(msgs []*gm.Message, detail MessageDetailLevel) (
	[]*gm.Message, error) {

	cache := h.getCache()

	newMsgs := make([]*gm.Message, 0, len(msgs))
	cachedMsgs := make([]*gm.Message, 0, len(msgs))
	for _, msg := range msgs {
		if cMsg, ok := cache.Msg(msg.Id); ok {
			cachedMsgs = append(cachedMsgs, cMsg)
		} else {
			newMsgs = append(newMsgs, msg)
		}
	}
	var err error
	newMsgs, err = h.fetchMessages(newMsgs, detail.Format())
	if err != nil {
		return nil, err
	}
	cache.UpdateMsgs(newMsgs)
	return append(cachedMsgs, newMsgs...), nil
}

func (h *MsgHelper) GetMessage(id string, detail MessageDetailLevel,
) (*gm.Message, error) {
	cache := h.getCache()
	cachedMsg, ok := cache.Msg(id)
	if ok && MessageMeetsDetailLevel(cachedMsg, detail) {
		// If the message was in the cache, and it was previously stored in full,
		// we can return it. Otherwise, we need to load the full version
		return cachedMsg, nil
	}

	return h.loadMessage(id, detail)
}

func (h *MsgHelper) loadMessage(id string, detail MessageDetailLevel,
) (*gm.Message, error) {
	msg, err := h.srv.Users.Messages.Get(h.User, id).Do()
	if err == nil {
		h.getCache().UpdateMsg(msg)
	}
	return msg, err
}

func (h *MsgHelper) getJustThread(id string) (*gm.Thread, error) {
	if preloadedThread, ok := h.loadedThreads[id]; ok {
		return preloadedThread, nil
	}
	// Note that this will never load the full message texts.
	thread, err := h.srv.Users.Threads.Get(h.User, id).Do()
	if err != nil {
		return nil, err
	}
	h.loadedThreads[thread.Id] = thread
	return thread, nil
}

// Loads the thread and caches its messages
func (h *MsgHelper) GetThread(id string, detail MessageDetailLevel,
) (*gm.Thread, error) {
	thread, err := h.getJustThread(id)
	for _, msg := range thread.Messages {
		// Simply pre-loads the messages into the cache at the desired level.
		_, err = h.GetMessage(msg.Id, detail)
		if err != nil {
			return nil, err
		}
	}
	return thread, nil
}

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
		msgs, err = h.fetchMessages(msgs, detailLevel.Format())
		if err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

func (h *MsgHelper) QueryThreads(
	query string, inboxOnly bool, unreadOnly bool, maxEntries int64,
	detailLevel MessageDetailLevel) ([]*gm.Thread, error) {

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
	// These threads will only have their own id, and not the message ids
	var threadShells []*gm.Thread

	for queriedPageCnt == 0 || pageToken != "" {
		queriedPageCnt++
		util.Debugf("Querying messages: '%s', page: %d\n", fullQuery, queriedPageCnt)

		call := h.srv.Users.Threads.List(h.User).Q(fullQuery)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		if maxEntries > 0 {
			call = call.MaxResults(maxEntries)
		}
		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("Unable to get messages: %v", err)
		}

		pageToken = r.NextPageToken

		for _, t := range r.Threads {
			threadShells = append(threadShells, t)
			if maxEntries > 0 && int64(len(threadShells)) == maxEntries {
				// Signal no more pages that we want to read.
				pageToken = ""
				break
			}
		}
	}

	threads, err := h.fetchThreads(threadShells, detailLevel)
	if err != nil {
		return nil, err
	}
	return threads, nil
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
