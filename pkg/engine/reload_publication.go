package engine

import (
	"errors"
	"fmt"
	"sort"
)

// ReloadPublication describes the runtime generation that has been installed
// into the engine and is now visible to public readers.
type ReloadPublication struct {
	Runtime Runtime
}

// ReloadPublicationListener observes a committed runtime generation. It must
// be cheap and non-blocking; broad repair work belongs in the scheduler lanes.
type ReloadPublicationListener func(ReloadPublication) error

type reloadPublicationListenerEntry struct {
	name     string
	listener ReloadPublicationListener
}

// RegisterReloadPublicationListener installs or replaces a named listener.
// Passing nil unregisters the listener for that name.
func (e *Engine) RegisterReloadPublicationListener(name string, listener ReloadPublicationListener) {
	if e == nil || name == "" {
		return
	}
	e.reloadPublicationListenersMu.Lock()
	defer e.reloadPublicationListenersMu.Unlock()

	if listener == nil {
		delete(e.reloadPublicationListeners, name)
		return
	}
	if e.reloadPublicationListeners == nil {
		e.reloadPublicationListeners = map[string]ReloadPublicationListener{}
	}
	e.reloadPublicationListeners[name] = listener
}

func (e *Engine) reloadPublicationListenerSnapshot() []reloadPublicationListenerEntry {
	if e == nil {
		return nil
	}
	e.reloadPublicationListenersMu.Lock()
	defer e.reloadPublicationListenersMu.Unlock()
	if len(e.reloadPublicationListeners) == 0 {
		return nil
	}

	names := make([]string, 0, len(e.reloadPublicationListeners))
	for name := range e.reloadPublicationListeners {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]reloadPublicationListenerEntry, 0, len(names))
	for _, name := range names {
		out = append(out, reloadPublicationListenerEntry{
			name:     name,
			listener: e.reloadPublicationListeners[name],
		})
	}
	return out
}

func (e *Engine) dispatchReloadPublication(pub ReloadPublication) error {
	listeners := e.reloadPublicationListenerSnapshot()
	if len(listeners) == 0 {
		return nil
	}
	var errs []error
	for _, entry := range listeners {
		if err := e.callReloadPublicationListener(entry, pub); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *Engine) callReloadPublicationListener(entry reloadPublicationListenerEntry, pub ReloadPublication) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("reload listener %q panicked: %v", entry.name, recovered)
			if e != nil && e.logger != nil {
				e.logger.Error("reload publication listener panicked", "listener", entry.name, "panic", recovered)
			}
		}
	}()

	if entry.listener == nil {
		return nil
	}
	if err := entry.listener(pub); err != nil {
		wrapped := fmt.Errorf("reload listener %q failed: %w", entry.name, err)
		if e != nil && e.logger != nil {
			e.logger.Error("reload publication listener failed", "listener", entry.name, "error", err)
		}
		return wrapped
	}
	return nil
}
