package api

import (
	"bufio"
	"encoding/gob"
	"os"
	"sync"

	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	msgCacheFileName = "msgcache.dat"

	maxCacheEntries = 1000
)

type Cache struct {
	// Maps of messages by ID
	storedMsgs map[string]*gm.Message
	newMsgs    map[string]*gm.Message
	useFile    bool
	closed     bool

	mutex sync.RWMutex
}

func NewCache(useFile bool) *Cache {
	cache := &Cache{
		storedMsgs: make(map[string]*gm.Message),
		newMsgs:    make(map[string]*gm.Message),
		useFile:    useFile,
	}
	gob.Register(gm.Message{})
	gob.Register(cache.storedMsgs)

	util.RegisterCleanupHandler(cache, func() {
		cache.Close()
	})

	return cache
}

func (c *Cache) msgCacheName() string {
	return util.RequiredHomeDirAndFile(util.UserAppDirName, msgCacheFileName)
}

func (c *Cache) cacheExists(fname string) bool {
	_, err := os.Stat(fname)
	return !os.IsNotExist(err)
}

func (c *Cache) Msg(id string) (*gm.Message, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	msg, ok := c.newMsgs[id]
	if ok {
		return msg, true
	}
	msg, ok = c.storedMsgs[id]
	if ok {
		return msg, true
	}
	return nil, false
}

func (c *Cache) LoadMsgs() {
	if !c.useFile {
		return
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()

	fname := c.msgCacheName()
	if !c.cacheExists(fname) {
		return
	}

	f, err := os.Open(fname)
	util.CheckErrf(err, "Error opening %s:", fname)
	defer f.Close()

	r := bufio.NewReader(f)
	dec := gob.NewDecoder(r)

	err = dec.Decode(&c.storedMsgs)
	util.CheckErr(err, "Cache decode error. "+
		"Please delete %s or re-run with --clear-cache:", fname)
}

func (c *Cache) writeStoredMsgs() {
	if !c.useFile {
		prnt.Deb.Ln("Cache::writeStoreMsgs: Skipping write to file")
		return
	}
	fname := c.msgCacheName()
	f, err := os.Create(fname)
	util.CheckErrf(err, "Error creating %s:", fname)
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := gob.NewEncoder(w)

	err = enc.Encode(c.storedMsgs)
	util.CheckErr(err, "Cache encode error:")
	w.Flush()
}

func (c *Cache) write() {
	if len(c.newMsgs) == 0 {
		prnt.Deb.Ln("Cache::WriteMsg no new messages to write")
		return
	}

	nOldMsgs := len(c.storedMsgs)
	// Move the new messages to the storedMsgs collection, and write them.
	nNewToStore := util.IntMin(len(c.newMsgs), maxCacheEntries)
	roomForOldMsgs := maxCacheEntries - nNewToStore
	nOldMsgsToDelete := util.IntMax(0, len(c.storedMsgs)-roomForOldMsgs)

	i := 0
	for id, _ := range c.storedMsgs {
		if i >= nOldMsgsToDelete {
			break
		}
		delete(c.storedMsgs, id)
		i += 1
	}

	i = 0
	for id, msg := range c.newMsgs {
		if i >= nNewToStore {
			break
		}
		c.storedMsgs[id] = msg
		delete(c.newMsgs, id)
		i += 1
	}

	prnt.Deb.F("Cache::WriteMsg: Stale cached messages: %d. "+
		"Removing %d stale messages, storing %d/%d new messages\n",
		nOldMsgs, nOldMsgsToDelete, nNewToStore, nNewToStore+len(c.newMsgs))
	c.writeStoredMsgs()
}

func (c *Cache) Write() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.write()
}

func (c *Cache) updateMsg(msg *gm.Message) {
	if _, ok := c.storedMsgs[msg.Id]; ok {
		delete(c.storedMsgs, msg.Id)
	}
	c.newMsgs[msg.Id] = msg
}

func (c *Cache) UpdateMsg(msg *gm.Message) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.updateMsg(msg)
}

func (c *Cache) UpdateMsgs(msgs []*gm.Message) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, msg := range msgs {
		c.updateMsg(msg)
	}
}

func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	fname := c.msgCacheName()
	if !c.cacheExists(fname) {
		return
	}
	err := os.Remove(fname)
	util.CheckErrf(err, "Unable to delete %s:", fname)
	prnt.LPrintf(prnt.Quietable, "Deleted %s\n", fname)
}

// Implements io.Closer interface
func (c *Cache) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.closed {
		return nil
	}
	prnt.Deb.F("%p Cache::Close\n", c)
	util.UnregisterCleanupHandler(c)
	c.write()
	c.closed = true
	return nil
}
