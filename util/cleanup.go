package util

// Handlers indexed by something, generally the object being cleaned up later.
var cleanupHandlers map[interface{}]func()

func RegisterCleanupHandler(k interface{}, f func()) {
	if cleanupHandlers == nil {
		cleanupHandlers = make(map[interface{}]func())
	}
	cleanupHandlers[k] = f
}

func UnregisterCleanupHandler(k interface{}) {
	delete(cleanupHandlers, k)
}

func RunCleanupHandlers() {
	if cleanupHandlers != nil {
		for _, h := range cleanupHandlers {
			h()
		}
	}
}
