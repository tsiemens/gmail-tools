package util

import "sync"

// Handlers indexed by something, generally the object being cleaned up later.
var cleanupHandlers map[interface{}]func()
var cleanupMutex sync.Mutex

func RegisterCleanupHandler(k interface{}, f func()) {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	cleanupHandlers[k] = f
}

func UnregisterCleanupHandler(k interface{}) {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	delete(cleanupHandlers, k)
}

func RunCleanupHandlers() {
	cleanupMutex.Lock()
	handlersCopy := make([]func(), 0, len(cleanupHandlers))
	for _, h := range cleanupHandlers {
		handlersCopy = append(handlersCopy, h)
	}
	cleanupMutex.Unlock()

	for _, h := range handlersCopy {
		h()
	}
}

func init() {
	cleanupHandlers = make(map[interface{}]func())
	cleanupMutex = sync.Mutex{}
}
