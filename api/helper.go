package api

import (
	"fmt"
	"log"
	"sync"

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

	useCacheFile bool
	cache        *Cache
	// These are not cached, because they can change between queries
	loadedThreads map[string]*gm.Thread
	mutex         sync.Mutex
}

func NewMsgHelper(user string, srv *gm.Service, useCacheFile bool) *MsgHelper {
	return &MsgHelper{
		User:          user,
		srv:           srv,
		useCacheFile:  useCacheFile,
		loadedThreads: make(map[string]*gm.Thread),
	}
}

func (h *MsgHelper) getCache() *Cache {
	if h.cache == nil {
		h.cache = NewCache(h.useCacheFile)
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
	lNames := make([]string, 0, len(ids))
	for _, lId := range ids {
		lNames = append(lNames, h.labels[lId])
	}
	return lNames
}

func (h *MsgHelper) MessageLabelNames(m *gm.Message) []string {
	return h.LabelNames(m.LabelIds)
}

func (h *MsgHelper) ThreadLabelNames(threadId string) ([]string, error) {
	thread, err := h.GetThread(threadId, IdsOnly)
	if err != nil {
		return nil, err
	}

	labelIdSet := make(map[string]bool) // Just used as a set
	// Grab the first and last message, since they should be fine to infer the labels
	// on the thread. Otherwise, very long threads take a long time to load.
	// The general case is when we receive a message, label it, and then a followup
	// appears, and we want to treat it with the same label.
	msgs := make([]*gm.Message, 0, 2)
	msgs = append(msgs, thread.Messages[0])
	if len(thread.Messages) > 1 {
		msgs = append(msgs, thread.Messages[len(thread.Messages)-1])
	}
	for _, msg := range msgs {
		msg, err = h.GetMessage(msg.Id, LabelsOnly)
		if err != nil {
			return nil, err
		}
		for _, lId := range msg.LabelIds {
			labelIdSet[lId] = true
		}
	}

	labelIds := make([]string, 0, len(labelIdSet))
	for lId := range labelIdSet {
		labelIds = append(labelIds, lId)
	}

	return h.LabelNames(labelIds), nil
}

func (h *MsgHelper) fetchMessages(msgs []*gm.Message, detail MessageDetailLevel) (
	[]*gm.Message, error) {

	prnt.Hum.Always.P("Loading message details ")

	concurrentQueries := MaxConcurrentRequests
	querySem := make(chan bool, concurrentQueries)
	msgChan := make(chan *gm.Message)
	errChan := make(chan error)

	for _, m_ := range msgs {
		go func(msg *gm.Message) {
			querySem <- true
			defer func() { <-querySem }()

			dMsg, err := h.GetMessage(msg.Id, detail)
			if err != nil {
				errChan <- fmt.Errorf("Failed to get message: %v", err)
			} else {
				msgChan <- dMsg
			}
		}(m_)
	}

	detailedMsgs := make([]*gm.Message, 0, len(msgs))
	errors := make([]error, 0)
	progP := prnt.NewProgressPrinter(len(msgs))
	for i := 0; i < len(msgs); i++ {
		progP.Progress(1)
		select {
		case msg := <-msgChan:
			detailedMsgs = append(detailedMsgs, msg)
		case err := <-errChan:
			errors = append(errors, err)
		}
	}
	prnt.Hum.Always.P("\n")

	if len(errors) > 0 {
		return nil, errors[0]
	}
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
	newMsgs, err = h.fetchMessages(newMsgs, detail)
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
	prnt.Deb.Ln("Loading msg", id, "at level", detail)
	msg, err := h.srv.Users.Messages.Get(h.User, id).Do()
	if err == nil {
		h.getCache().UpdateMsg(msg)
	}
	return msg, err
}

func (h *MsgHelper) ThreadIsLoaded(id string) bool {
	_, ok := h.loadedThreads[id]
	return ok
}

func (h *MsgHelper) getJustThread(id string) (*gm.Thread, error) {
	h.mutex.Lock()
	preloadedThread, ok := h.loadedThreads[id]
	h.mutex.Unlock()
	if ok {
		return preloadedThread, nil
	}

	// Note that this will never load the full message texts.
	thread, err := h.srv.Users.Threads.Get(h.User, id).Do()
	if err != nil {
		return nil, err
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.loadedThreads[thread.Id] = thread
	return thread, nil
}

// Loads the thread and caches its messages
func (h *MsgHelper) GetThread(id string, detail MessageDetailLevel,
) (*gm.Thread, error) {
	thread, err := h.getJustThread(id)
	if err != nil {
		return nil, err
	}
	if detail != IdsOnly {
		for _, msg := range thread.Messages {
			// Simply pre-loads the messages into the cache at the desired level.
			_, err = h.GetMessage(msg.Id, detail)
			if err != nil {
				return nil, err
			}
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
		msgs, err = h.fetchMessages(msgs, detailLevel)
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

func (h *MsgHelper) LabelIdForLabel(l *Label) string {
	if l.id != "" {
		return l.id
	}
	return h.LabelIdFromName(l.name)
}

// If labels is nil or empty, returns nil
func (h *MsgHelper) LabelIdsForLabels(labels []Label) []string {
	var labelIds []string = nil
	if labels != nil && len(labels) > 0 {
		labelIds = make([]string, 0, len(labels))
		for _, label := range labels {
			labelIds = append(labelIds, h.LabelIdForLabel(&label))
		}
	}
	return labelIds
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

type SizedMessageIdIterator interface {
	Next() (string, bool)
	Len() int
}

type MessageListIterator struct {
	messages []*gm.Message
	index    int
}

func (ml *MessageListIterator) Next() (string, bool) {
	if ml.index >= len(ml.messages) {
		return "", false
	}
	id := ml.messages[ml.index].Id
	ml.index++
	return id, true
}

func SizedMessageIdIteratorFromMsgs(msgs []*gm.Message) SizedMessageIdIterator {
	return &MessageListIterator{messages: msgs}
}

func (ml *MessageListIterator) Len() int {
	return len(ml.messages)
}

type MessageIdListIterator struct {
	ids   []string
	index int
}

func (mil *MessageIdListIterator) Next() (string, bool) {
	if mil.index >= len(mil.ids) {
		return "", false
	}
	id := mil.ids[mil.index]
	mil.index++
	return id, true
}

func (mil *MessageIdListIterator) Len() int {
	return len(mil.ids)
}

func SizedMessageIdIteratorFromIds(ids []string) SizedMessageIdIterator {
	return &MessageIdListIterator{ids: ids}
}

func (h *MsgHelper) BatchModifyByIdIter(
	iterator SizedMessageIdIterator, modReq *gm.BatchModifyMessagesRequest) error {
	var err error
	nItems := iterator.Len()
	itemsLeft := nItems

	nBatches := nItems / MaxBatchModifySize
	if nItems%MaxBatchModifySize != 0 {
		nBatches++
	}

	prnt.HPrintf(prnt.Quietable, "Applying changes to ")
	for batch := 0; batch < nBatches; batch++ {
		batchSize := util.IntMin(itemsLeft, MaxBatchModifySize)
		prnt.HPrintf(prnt.Quietable, "%d... ", nItems-itemsLeft+batchSize)

		ids := make([]string, 0, batchSize)
		for i := 0; i < batchSize; i++ {
			id, ok := iterator.Next()
			if !ok {
				break
			}
			ids = append(ids, id)
		}

		modReq.Ids = ids
		err = h.srv.Users.Messages.BatchModify(h.User, modReq).Do()
		if err != nil {
			return err
		}

		itemsLeft -= batchSize
	}
	prnt.HPrintf(prnt.Quietable, "Done\n")

	return nil
}

func (h *MsgHelper) BatchModifyMessages(msgs []*gm.Message, modReq *gm.BatchModifyMessagesRequest) error {
	iterator := SizedMessageIdIteratorFromMsgs(msgs)
	return h.BatchModifyByIdIter(iterator, modReq)
}

func (h *MsgHelper) BatchModifyMessagesByIds(ids []string, modReq *gm.BatchModifyMessagesRequest) error {
	iterator := SizedMessageIdIteratorFromIds(ids)
	return h.BatchModifyByIdIter(iterator, modReq)
}

func (h *MsgHelper) ApplyLabels(
	msgs []*gm.Message, labelsToAdd []Label, labelsToRemove []Label) error {
	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds:    h.LabelIdsForLabels(labelsToAdd),
		RemoveLabelIds: h.LabelIdsForLabels(labelsToRemove),
	}
	return h.BatchModifyMessages(msgs, &modReq)
}

func (h *MsgHelper) ApplyLabelsByIdIter(
	msgs SizedMessageIdIterator, labelsToAdd []Label, labelsToRemove []Label) error {
	modReq := gm.BatchModifyMessagesRequest{
		AddLabelIds:    h.LabelIdsForLabels(labelsToAdd),
		RemoveLabelIds: h.LabelIdsForLabels(labelsToRemove),
	}
	return h.BatchModifyByIdIter(msgs, &modReq)
}

func (h *MsgHelper) ApplyLabelsToThreads(threads []*gm.Thread, labelNames []string) error {
	allMsgs := make([]*gm.Message, 0, len(threads))
	for _, thread := range threads {
		for _, msg := range thread.Messages {
			allMsgs = append(allMsgs, msg)
		}
	}

	return h.ApplyLabels(allMsgs, LabelsFromLabelNames(labelNames), nil)
}
