package api

import (
	"bufio"
	"encoding/gob"
	"os"

	gm "google.golang.org/api/gmail/v1"

	"github.com/tsiemens/gmail-tools/prnt"
	"github.com/tsiemens/gmail-tools/util"
)

const (
	msgCacheFileName = "msgcache.dat"

	maxCacheEntries = 1000
)

type Cache struct {
	// Map of messages by ID
	Msgs      map[string]*gm.Message
	msgsDirty bool
}

func NewCache() *Cache {
	cache := &Cache{Msgs: make(map[string]*gm.Message), msgsDirty: false}
	gob.Register(gm.Message{})
	gob.Register(cache.Msgs)
	return cache
}

func (c *Cache) msgCacheName() string {
	return util.RequiredHomeDirAndFile(util.UserAppDirName, msgCacheFileName)
}

func (c *Cache) cacheExists(fname string) bool {
	_, err := os.Stat(fname)
	return !os.IsNotExist(err)
}

func (c *Cache) LoadMsgs() {
	fname := c.msgCacheName()
	if !c.cacheExists(fname) {
		return
	}

	f, err := os.Open(fname)
	util.CheckErrf(err, "Error opening %s:", fname)
	defer f.Close()

	r := bufio.NewReader(f)
	dec := gob.NewDecoder(r)

	err = dec.Decode(&c.Msgs)
	util.CheckErr(err, "Cache decode error. "+
		"Please delete %s or re-run with --clear-cache:", fname)
}

func (c *Cache) WriteMsgs() {
	if !c.msgsDirty {
		return
	}

	fname := c.msgCacheName()
	f, err := os.Create(fname)
	util.CheckErrf(err, "Error creating %s:", fname)
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := gob.NewEncoder(w)

	err = enc.Encode(c.Msgs)
	util.CheckErr(err, "Cache encode error:")
	w.Flush()
	c.msgsDirty = false
}

func (c *Cache) UpdateMsgs(msgs []*gm.Message) {
	oldMsgIds := make(map[string]bool, len(c.Msgs))

	i := 0
	for _, msg := range msgs {
		if i >= maxCacheEntries {
			break
		}
		c.Msgs[msg.Id] = msg
		delete(oldMsgIds, msg.Id)
		i++
	}

	if len(c.Msgs) > maxCacheEntries {
		for id, _ := range oldMsgIds {
			delete(c.Msgs, id)
			if len(c.Msgs) <= maxCacheEntries {
				break
			}
		}
	}

	c.msgsDirty = true
}

func (c *Cache) Clear() {
	fname := c.msgCacheName()
	if !c.cacheExists(fname) {
		return
	}
	err := os.Remove(fname)
	util.CheckErrf(err, "Unable to delete %s:", fname)
	prnt.LPrintf(prnt.Quietable, "Deleted %s\n", fname)
}
