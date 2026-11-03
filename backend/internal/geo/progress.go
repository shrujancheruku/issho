package geo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Progress is the record of finished Nearby Searches that lets a rerun after
// a failure skip (and not pay for) what the failed run did. It lives in a
// local file, not the database: a search that found nothing leaves no row to
// find it by, and a neighbour's search can leave venues in an area whose own
// search never ran.
//
// A search is recorded as the run makes it but only reaches the file at
// Commit, which the caller calls once the run's venues are saved. So a listed
// search always has its venues stored, and if saving fails nothing is listed.
// Each entry says whether the search came back full, which is what decides
// whether its seven finer searches follow (split), so a skipped search still
// leads to its children. A nil *Progress records and skips nothing.
type Progress struct {
	path string

	mu      sync.Mutex
	done    map[string]bool // search key → came back full
	pending map[string]bool
}

type progressFile struct {
	Searches map[string]bool `json:"searches"`
}

// LoadProgress reads the file at path; a missing file is an empty record.
// fresh ignores what the file holds (the next Commit replaces it).
func LoadProgress(path string, fresh bool) (*Progress, error) {
	p := &Progress{path: path, done: map[string]bool{}, pending: map[string]bool{}}
	if fresh {
		return p, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	var f progressFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("progress file %s: %w (delete it or pass -fresh)", path, err)
	}
	for k, v := range f.Searches {
		p.done[k] = v
	}
	return p, nil
}

// searchKey identifies one search: the same plan gives the same keys, down
// to the circles split makes (rounded to about a metre).
func searchKey(j VenueJob) string {
	return fmt.Sprintf("%s|%s|%.5f,%.5f|%.0f", j.Category.ID, j.From, j.Center.Lat, j.Center.Lng, j.RadiusM)
}

func (p *Progress) finished(key string) (full, ok bool) {
	if p == nil {
		return false, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	full, ok = p.done[key]
	return full, ok
}

func (p *Progress) record(key string, full bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.pending[key] = full
	p.mu.Unlock()
}

// Finished counts the first searches of a plan that the record lists: what a
// dry run reports as skipped (finer ones are known only as the run goes).
func (p *Progress) Finished(jobs []VenueJob) int {
	n := 0
	for _, j := range jobs {
		if _, ok := p.finished(searchKey(j)); ok {
			n++
		}
	}
	return n
}

// Commit adds the run's recorded searches to the file, atomically (a temp
// file renamed over it), so a crash never leaves half a file.
func (p *Progress) Commit() error {
	if p == nil || p.path == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, v := range p.pending {
		p.done[k] = v
	}
	p.pending = map[string]bool{}
	b, err := json.Marshal(progressFile{Searches: p.done})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".progress-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), p.path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Clear removes the file: the run finished, so the next one starts over.
func (p *Progress) Clear() error {
	if p == nil || p.path == "" {
		return nil
	}
	if err := os.Remove(p.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
