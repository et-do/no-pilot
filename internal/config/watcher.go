package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const reloadDebounce = 75 * time.Millisecond

// Watcher keeps a live Config snapshot and reloads it when policy files change.
// It is safe for concurrent use by tool handlers.
type Watcher struct {
	projectDir string
	userCfg    string
	projCfg    string
	logger     *log.Logger

	mu   sync.RWMutex
	cfg  *Config
	done chan struct{}
	wg   sync.WaitGroup

	watchedDirs map[string]struct{}

	fsw *fsnotify.Watcher
}

// NewWatcher loads the initial config and starts a background fsnotify loop.
// The caller must call Close to release watcher resources.
func NewWatcher(projectDir string, logger *log.Logger) (*Watcher, error) {
	cfg, err := Load(projectDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create file watcher: %w", err)
	}

	w := &Watcher{
		projectDir:  projectDir,
		userCfg:     userConfigPath(),
		projCfg:     filepath.Join(projectDir, ProjectPolicyFileName),
		logger:      logger,
		cfg:         cfg,
		done:        make(chan struct{}),
		watchedDirs: map[string]struct{}{},
		fsw:         fsw,
	}

	if err := w.watchPaths(); err != nil {
		_ = w.fsw.Close()
		return nil, err
	}

	w.wg.Add(1)
	go w.loop()

	if w.logger != nil {
		w.logger.Printf("config loaded")
	}
	return w, nil
}

// Policy returns the latest effective policy for tool.
func (w *Watcher) Policy(tool string) ToolPolicy {
	w.mu.RLock()
	cfg := w.cfg
	w.mu.RUnlock()
	if cfg == nil {
		return ToolPolicy{}
	}
	return cfg.Policy(tool)
}

// Close stops the watcher goroutine and releases fsnotify resources.
func (w *Watcher) Close() error {
	close(w.done)
	w.wg.Wait()
	return w.fsw.Close()
}

func (w *Watcher) loop() {
	defer w.wg.Done()

	var (
		timer  *time.Timer
		timerC <-chan time.Time
	)

	scheduleReload := func() {
		if timer == nil {
			timer = time.NewTimer(reloadDebounce)
			timerC = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(reloadDebounce)
		timerC = timer.C
	}

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case <-timerC:
			w.reload()
			timerC = nil
		case evt, ok := <-w.fsw.Events:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			w.refreshWatchDirs()
			if !w.isConfigEvent(evt) {
				continue
			}
			scheduleReload()
		case err, ok := <-w.fsw.Errors:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			if w.logger != nil {
				w.logger.Printf("config watch error: %v", err)
			}
		}
	}
}

func (w *Watcher) reload() {
	cfg, err := Load(w.projectDir)
	if err != nil {
		if w.logger != nil {
			w.logger.Printf("config reload failed: %v", err)
		}
		return
	}

	w.mu.Lock()
	w.cfg = cfg
	w.mu.Unlock()

	if w.logger != nil {
		w.logger.Printf("config reloaded")
	}
}

func (w *Watcher) isConfigEvent(evt fsnotify.Event) bool {
	if evt.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}

	eventPath := filepath.Clean(evt.Name)
	return eventPath == filepath.Clean(w.userCfg) || eventPath == filepath.Clean(w.projCfg)
}

func (w *Watcher) watchPaths() error {
	userWatchDir, err := nearestExistingDir(filepath.Dir(w.userCfg))
	if err != nil {
		return fmt.Errorf("resolve user config watch dir: %w", err)
	}
	if err := w.addWatchDir(userWatchDir); err != nil {
		return err
	}

	projWatchDir, err := nearestExistingDir(filepath.Dir(w.projCfg))
	if err != nil {
		return fmt.Errorf("resolve project config watch dir: %w", err)
	}
	if err := w.addWatchDir(projWatchDir); err != nil {
		return err
	}

	w.refreshWatchDirs()
	return nil
}

func (w *Watcher) refreshWatchDirs() {
	_ = w.addWatchDir(filepath.Dir(w.userCfg))
	_ = w.addWatchDir(filepath.Dir(w.projCfg))
}

func (w *Watcher) addWatchDir(dir string) error {
	cleanDir := filepath.Clean(dir)
	if _, ok := w.watchedDirs[cleanDir]; ok {
		return nil
	}

	info, err := os.Stat(cleanDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat watch dir %s: %w", cleanDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch path %s is not a directory", cleanDir)
	}

	if err := w.fsw.Add(cleanDir); err != nil {
		return fmt.Errorf("watch dir %s: %w", cleanDir, err)
	}
	w.watchedDirs[cleanDir] = struct{}{}
	return nil
}

func nearestExistingDir(dir string) (string, error) {
	cur := filepath.Clean(dir)
	for {
		info, err := os.Stat(cur)
		if err == nil {
			if info.IsDir() {
				return cur, nil
			}
			return "", fmt.Errorf("path %s exists but is not a directory", cur)
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor for %s", dir)
		}
		cur = parent
	}
}
