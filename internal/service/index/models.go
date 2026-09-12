/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package index maintains the concurrent repository file index.
package index

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"
	"github.com/llxisdsh/pb"

	"renop/internal/cache/ttl"
	"renop/internal/utils"
)

func toSlashFast(pathStr string) string {
	if strings.IndexByte(pathStr, '\\') == -1 {
		return pathStr
	}
	return filepath.ToSlash(pathStr)
}

func cleanPathFast(pathStr string) string {
	return path.Clean(toSlashFast(pathStr))
}

type FileInfo struct {
	Size    int64 `json:"size"`
	ModTime int64 `json:"mod_time"`
	// Revision distinguishes observed replacements even when storage reports
	// identical sizes and timestamps. It is local to one index lifetime.
	Revision uint64 `json:"-"`
}

func sameFileMetadata(a, b FileInfo) bool { return a.Size == b.Size && a.ModTime == b.ModTime }

func (idx *FileIndex) ReadJSONFrom(r io.Reader) error {
	decoder := json.NewDecoder(r)
	version := 0
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := opening.(json.Delim); !ok || delim != '{' {
		return errors.New("index root must be an object")
	}

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("index field name must be a string")
		}
		switch key {
		case "version":
			if err := decoder.Decode(&version); err != nil {
				return err
			}
			if version != 2 {
				return errors.New("unsupported index stream version")
			}
		case "files":
			if err := readFilesFromJSON(decoder, idx); err != nil {
				return err
			}
		case "dirs":
			if err := readDirsFromJSON(decoder, idx); err != nil {
				return err
			}
		case "not_found":
			if err := readNotFoundFromJSON(decoder, idx); err != nil {
				return err
			}
		case "content", "content_garbage":
			if err := idx.readContentJSON(decoder, key == "content_garbage"); err != nil {
				return err
			}
		default:
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return err
			}
		}
	}
	_, err = decoder.Token()
	if err == nil && version == 2 {
		return idx.readJSONStream(decoder)
	}
	return err
}

func readFilesFromJSON(decoder *json.Decoder, idx *FileIndex) error {
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := opening.(json.Delim)
	if !ok || (delim != '{' && delim != '[') {
		return errors.New("index files must be an object or array")
	}
	if delim == '{' {
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			filePath, ok := keyToken.(string)
			if !ok {
				return errors.New("index file path must be a string")
			}
			filePath = utils.Intern(filePath)
			var info FileInfo
			if err := decoder.Decode(&info); err != nil {
				return err
			}
			idx.InsertFile(filePath, info)
		}
		_, err = decoder.Token()
		return err
	}

	for decoder.More() {
		var filePath string
		if err := decoder.Decode(&filePath); err != nil {
			return err
		}
		filePath = utils.Intern(filePath)
		var info FileInfo
		if stat, statErr := os.Stat(filepath.FromSlash(filePath)); statErr == nil {
			info = FileInfo{Size: stat.Size(), ModTime: stat.ModTime().UnixNano()}
		}
		idx.InsertFile(filePath, info)
	}
	_, err = decoder.Token()
	return err
}

func readDirsFromJSON(decoder *json.Decoder, idx *FileIndex) error {
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := opening.(json.Delim); !ok || delim != '[' {
		return errors.New("index dirs must be an array")
	}
	for decoder.More() {
		var dir string
		if err := decoder.Decode(&dir); err != nil {
			return err
		}
		idx.InsertDir(utils.Intern(dir))
	}
	_, err = decoder.Token()
	return err
}

func readNotFoundFromJSON(decoder *json.Decoder, idx *FileIndex) error {
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := opening.(json.Delim); !ok || delim != '{' {
		return errors.New("index not_found must be an object")
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		filePath, ok := keyToken.(string)
		if !ok {
			return errors.New("negative cache path must be a string")
		}
		var expireAt int64
		if err := decoder.Decode(&expireAt); err != nil {
			return err
		}
		idx.InsertNotFound(utils.Intern(filePath), expireAt)
	}
	_, err = decoder.Token()
	return err
}

type FileIndex struct {
	contentMu           sync.RWMutex
	contents            pb.MapOf[string, ContentInfo]
	contentRefs         map[string]int
	contentVerifiedRefs map[string]int
	contentGarbage      pb.MapOf[string, int64]
	Files               pb.MapOf[string, FileInfo] `json:"-"`
	Dirs                pb.MapOf[string, bool]     `json:"-"`
	Blocked             pb.MapOf[string, bool]     `json:"-"`
	Children            map[string][]string        `json:"-"`
	ChildrenMutex       sync.RWMutex               `json:"-"`
	FilesCount          atomic.Uint64              `json:"-"`
	DirsCount           atomic.Uint64              `json:"-"`
	TotalBytes          atomic.Int64               `json:"-"` // sum of FileInfo.Size; O(1) disk usage
	fileRevision        atomic.Uint64
	negativeOnce        sync.Once
	negative            *ttl.TTLCache[string, int64]
	IsDirty             atomic.Bool `json:"-"`

	mutationMu     sync.Mutex
	metadataLock   sync.Mutex           `json:"-"`
	rebuildMu      sync.Mutex           `json:"-"`
	rebuildRunning bool                 `json:"-"`
	rebuildNext    *indexRebuildRequest `json:"-"`
}

func internString(s string) string {
	return utils.Intern(s)
}

func splitPathParentBase(filePath string) (string, string) {
	idx := strings.LastIndexByte(filePath, '/')
	if idx <= 0 {
		if idx == 0 {
			return "/", filePath[1:]
		}
		return "", filePath
	}
	return filePath[:idx], filePath[idx+1:]
}

func (idx *FileIndex) addChild(filePath string) {
	parent, base := splitPathParentBase(filePath)
	if parent == filePath || parent == "." || parent == "/" || parent == "" {
		return
	}

	parentInterned := internString(parent)
	baseInterned := internString(base)

	idx.ChildrenMutex.RLock()
	if idx.Children != nil {
		if list, ok := idx.Children[parentInterned]; ok {
			if slices.Contains(list, baseInterned) {
				idx.ChildrenMutex.RUnlock()
				return
			}
		}
	}
	idx.ChildrenMutex.RUnlock()

	idx.ChildrenMutex.Lock()
	defer idx.ChildrenMutex.Unlock()
	if idx.Children == nil {
		idx.Children = make(map[string][]string)
	}
	list := idx.Children[parentInterned]
	if slices.Contains(list, baseInterned) {
		return
	}
	idx.Children[parentInterned] = append(list, baseInterned)
}

func (idx *FileIndex) removeChild(filePath string) {
	parent, base := splitPathParentBase(filePath)

	idx.ChildrenMutex.Lock()
	defer idx.ChildrenMutex.Unlock()
	if idx.Children == nil {
		return
	}
	if list, ok := idx.Children[parent]; ok {
		for i, item := range list {
			if item == base {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(list) == 0 {
			delete(idx.Children, parent)
		} else {
			idx.Children[parent] = list
		}
	}
	delete(idx.Children, filePath)
}

func NewFileIndex() *FileIndex { return &FileIndex{} }

// putFile stores or replaces a file entry and maintains FilesCount / TotalBytes.
func (idx *FileIndex) putFile(pathSlash string, info FileInfo) {
	pathSlash = utils.Intern(pathSlash)
	if idx.IsBlocked(pathSlash) {
		return
	}
	info.Revision = idx.fileRevision.Add(1)
	if _, loaded := idx.Dirs.LoadAndDelete(pathSlash); loaded {
		idx.removeDescendants(pathSlash)
		idx.DirsCount.Add(^uint64(0))
		idx.removeChild(pathSlash)
		idx.IsDirty.Store(true)
	}
	old, loaded := idx.Files.LoadOrStore(pathSlash, info)
	if !loaded {
		idx.FilesCount.Add(1)
		if info.Size != 0 {
			idx.TotalBytes.Add(info.Size)
		}
		idx.IsDirty.Store(true)
		idx.addChild(pathSlash)
		return
	}
	if old.Size != info.Size {
		idx.TotalBytes.Add(info.Size - old.Size)
	}
	idx.Files.Store(pathSlash, info)
	if !sameFileMetadata(old, info) {
		idx.IsDirty.Store(true)
	}
	idx.addChild(pathSlash)
}

// putDir stores a directory unless the same path is already an indexed file.
func (idx *FileIndex) putDir(pathSlash string) {
	pathSlash = utils.Intern(pathSlash)
	if _, isFile := idx.Files.Load(pathSlash); isFile {
		return
	}
	if _, loaded := idx.Dirs.LoadOrStore(pathSlash, true); !loaded {
		idx.DirsCount.Add(1)
		idx.IsDirty.Store(true)
		idx.addChild(pathSlash)
	}
}

// deleteFile removes a file entry and maintains FilesCount / TotalBytes.
func (idx *FileIndex) deleteFile(pathSlash string) bool {
	pathSlash = utils.Intern(pathSlash)
	old, loaded := idx.Files.LoadAndDelete(pathSlash)
	if !loaded {
		return false
	}
	idx.FilesCount.Add(^uint64(0))
	if old.Size != 0 {
		idx.TotalBytes.Add(-old.Size)
	}
	idx.removeChild(pathSlash)
	return true
}

// TotalFileBytes returns the sum of indexed file sizes (clamped to zero).
func (idx *FileIndex) TotalFileBytes() uint64 {
	n := idx.TotalBytes.Load()
	if n <= 0 {
		return 0
	}
	return uint64(n)
}

func (idx *FileIndex) HasFile(pathStr string) bool {
	pathStr = toSlashFast(pathStr)
	if idx.IsBlocked(pathStr) {
		return false
	}
	_, ok := idx.Files.Load(pathStr)
	return ok
}

func (idx *FileIndex) HasDir(pathStr string) bool {
	_, ok := idx.Dirs.Load(toSlashFast(pathStr))
	return ok
}

func (idx *FileIndex) GetFileInfo(pathStr string) (FileInfo, bool) {
	pathStr = toSlashFast(pathStr)
	if idx.IsBlocked(pathStr) {
		return FileInfo{}, false
	}
	val, ok := idx.Files.Load(pathStr)
	return val, ok
}

func (idx *FileIndex) GetPathState(pathStr string) (isDir bool, info FileInfo, ok bool, isNotFound bool) {
	pathStr = toSlashFast(pathStr)
	if idx.IsBlocked(pathStr) {
		return false, FileInfo{}, false, true
	}

	if val, exists := idx.Files.Load(pathStr); exists {
		return false, val, true, false
	}

	if _, exists := idx.Dirs.Load(pathStr); exists {
		return true, FileInfo{}, true, false
	}

	return false, FileInfo{}, false, idx.IsNotFound(pathStr)
}

func (idx *FileIndex) InsertFile(pathStr string, info ...FileInfo) {
	idx.mutationMu.Lock()
	pathStr = toSlashFast(pathStr)
	if idx.IsBlocked(pathStr) {
		idx.mutationMu.Unlock()
		return
	}
	var fileInfo FileInfo
	if len(info) > 0 {
		fileInfo = info[0]
	}

	wasDir := idx.HasDir(pathStr)
	idx.putFile(pathStr, fileInfo)
	idx.mutationMu.Unlock()
	if wasDir {
		idx.clearNotFoundTree(pathStr)
	} else {
		idx.clearNotFound(pathStr)
	}
}

// BlockFile hides a physical object from every index insertion path until it
// is explicitly unblocked. Durability is provided by the release database;
// the in-memory set is restored before storage watchers and rebuilds start.
func (idx *FileIndex) BlockFile(pathStr string) {
	if idx == nil || pathStr == "" {
		return
	}
	pathStr = utils.Intern(toSlashFast(pathStr))
	idx.mutationMu.Lock()
	idx.Blocked.Store(pathStr, true)
	if idx.deleteFile(pathStr) {
		idx.IsDirty.Store(true)
	}
	idx.mutationMu.Unlock()
	idx.clearNotFound(pathStr)
}

// UnblockFile permits a successfully published or fully cleaned path to be
// indexed again. It does not insert the path by itself.
func (idx *FileIndex) UnblockFile(pathStr string) {
	if idx == nil || pathStr == "" {
		return
	}
	idx.Blocked.Delete(utils.Intern(toSlashFast(pathStr)))
}

// UnblockTree releases a deleted publication tree after its durable records
// have been removed. Watcher removals must not call this: pending jobs may still
// need their blocks even when the physical directory temporarily disappears.
func (idx *FileIndex) UnblockTree(pathStr string) {
	if idx == nil || pathStr == "" {
		return
	}
	pathStr = cleanPathFast(pathStr)
	prefix := pathStr + "/"
	idx.mutationMu.Lock()
	defer idx.mutationMu.Unlock()
	idx.Blocked.Range(func(key string, _ bool) bool {
		if key == pathStr || strings.HasPrefix(key, prefix) {
			idx.Blocked.Delete(key)
		}
		return true
	})
}

func (idx *FileIndex) IsBlocked(pathStr string) bool {
	if idx == nil || pathStr == "" {
		return false
	}
	_, blocked := idx.Blocked.Load(toSlashFast(pathStr))
	return blocked
}

func (idx *FileIndex) InsertDir(pathStr string) {
	idx.mutationMu.Lock()
	pathStr = utils.Intern(toSlashFast(pathStr))

	idx.putDir(pathStr)
	idx.mutationMu.Unlock()
	idx.clearNotFound(pathStr)
}

func (idx *FileIndex) RemoveFile(pathStr string) {
	idx.mutationMu.Lock()
	pathStr = toSlashFast(pathStr)

	if idx.deleteFile(pathStr) {
		idx.IsDirty.Store(true)
	}
	idx.mutationMu.Unlock()
	idx.clearNotFound(pathStr)
}

func (idx *FileIndex) RemoveDir(pathStr string) {
	idx.mutationMu.Lock()
	pathStr = toSlashFast(pathStr)

	// Purge descendants first while Children[path] still lists them.
	// removeChild deletes that entry, so it must run after removeDescendants.
	idx.removeDescendants(pathStr)
	if _, loaded := idx.Dirs.LoadAndDelete(pathStr); loaded {
		idx.DirsCount.Add(^uint64(0))
		idx.IsDirty.Store(true)
	}
	idx.removeChild(pathStr)
	idx.mutationMu.Unlock()
	idx.clearNotFoundTree(pathStr)
}

func (idx *FileIndex) clearNotFound(pathStr string) {
	idx.negativeCache().Delete(pathStr)
}

func (idx *FileIndex) removeDescendants(dirPath string) {
	dirCleaned := cleanPathFast(dirPath)
	children := idx.GetChildren(dirCleaned)
	for _, child := range children {
		childPath := dirCleaned + "/" + child
		if idx.deleteFile(childPath) {
			idx.IsDirty.Store(true)
		}
		if _, loaded := idx.Dirs.LoadAndDelete(childPath); loaded {
			idx.DirsCount.Add(^uint64(0))
			idx.removeDescendants(childPath)
			idx.removeChild(childPath)
		}
	}

}

func (idx *FileIndex) clearNotFoundTree(dirPath string) {
	dirCleaned := cleanPathFast(dirPath)

	prefix := dirCleaned + "/"
	idx.negativeCache().DeleteFunc(func(k string, _ int64) bool {
		return k == dirCleaned || strings.HasPrefix(k, prefix)
	})
}

func (idx *FileIndex) InsertNotFound(pathStr string, expireAt int64) {
	pathStr = toSlashFast(pathStr)

	idx.storeNotFound(pathStr, expireAt)
}

func (idx *FileIndex) PruneNotFound() {
	idx.negativeCache().EvictExpired()
}

func (idx *FileIndex) UpdateMetadataCallback(cb func() error) error {
	idx.metadataLock.Lock()
	defer idx.metadataLock.Unlock()
	return cb()
}

// WriteJSONTo emits versioned JSON records without assembling the full index.
func (idx *FileIndex) WriteJSONTo(w io.Writer) error { return idx.writeJSONStream(w, false) }

// WritePersistentJSONTo also saves private content metadata for backend reuse.
func (idx *FileIndex) WritePersistentJSONTo(w io.Writer) error { return idx.writeJSONStream(w, true) }

type FileIndexSnapshot struct {
	Files    map[string]FileInfo `json:"files"`
	Dirs     []string            `json:"dirs"`
	NotFound map[string]int64    `json:"not_found"`
}

func (idx *FileIndex) IsNotFound(pathStr string) bool {
	pathStr = toSlashFast(pathStr)
	if idx.HasFile(pathStr) || idx.HasDir(pathStr) {
		return false
	}
	expires, ok := idx.negativeCache().Get(pathStr)
	return ok && time.Now().Unix() < expires
}

func (idx *FileIndex) EnsureParentDirs(basePath string) {
	basePath = toSlashFast(basePath)
	var pathsToInsert []string
	currentPath := basePath
	for {
		parent := path.Dir(currentPath)
		if parent == currentPath || parent == "." || parent == "/" || parent == "" {
			break
		}
		if idx.HasDir(parent) {
			break
		}
		pathsToInsert = append(pathsToInsert, parent)
		currentPath = parent
	}
	for _, p := range pathsToInsert {
		idx.InsertDir(p)
	}
}

func (idx *FileIndex) Snapshot() FileIndexSnapshot {
	idx.mutationMu.Lock()
	defer idx.mutationMu.Unlock()
	files := make(map[string]FileInfo)
	idx.Files.Range(func(k string, v FileInfo) bool {
		if idx.IsBlocked(k) {
			return true
		}
		files[k] = v
		return true
	})

	var dirs []string
	idx.Dirs.Range(func(k string, _ bool) bool {
		dirs = append(dirs, k)
		return true
	})

	notFounds := make(map[string]int64)
	idx.negativeCache().RangeIndex(func(k string, v int64) bool {
		notFounds[k] = v
		return true
	})

	return FileIndexSnapshot{
		Files:    files,
		Dirs:     dirs,
		NotFound: notFounds,
	}
}

func (idx *FileIndex) MarshalJSON() ([]byte, error) {
	snapshot := idx.Snapshot()
	return json.Marshal(struct {
		Files map[string]FileInfo `json:"files"`
		Dirs  []string            `json:"dirs"`
	}{snapshot.Files, snapshot.Dirs})
}

type fileIndexWire struct {
	Files    json.RawMessage  `json:"files"`
	Dirs     []string         `json:"dirs"`
	NotFound map[string]int64 `json:"not_found"`
}

func (idx *FileIndex) UnmarshalJSON(data []byte) error {
	var raw fileIndexWire

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for _, dir := range raw.Dirs {
		dirSlash := utils.Intern(filepath.ToSlash(dir))
		idx.Dirs.Store(dirSlash, true)
		idx.DirsCount.Add(1)
		idx.addChild(dirSlash)
	}

	for pathStr, expireAt := range raw.NotFound {
		idx.storeNotFound(toSlashFast(pathStr), expireAt)
	}

	if len(raw.Files) > 0 {
		if raw.Files[0] == '[' {
			var fileList []string
			if err := json.Unmarshal(raw.Files, &fileList); err != nil {
				return err
			}
			for _, file := range fileList {
				fileSlash := utils.Intern(filepath.ToSlash(file))
				var size int64
				var modTime int64
				if info, err := os.Stat(fileSlash); err == nil {
					size = info.Size()
					modTime = info.ModTime().UnixNano()
				}
				idx.putFile(fileSlash, FileInfo{Size: size, ModTime: modTime})
			}
		} else if raw.Files[0] == '{' {
			var fileMap map[string]FileInfo
			if err := json.Unmarshal(raw.Files, &fileMap); err != nil {
				return err
			}
			for file, info := range fileMap {
				idx.putFile(utils.Intern(filepath.ToSlash(file)), info)
			}
		}
	}

	return nil
}

func (idx *FileIndex) GetChildren(dirPath string) []string {
	dirPath = cleanPathFast(dirPath)
	idx.ChildrenMutex.RLock()
	defer idx.ChildrenMutex.RUnlock()
	if idx.Children == nil {
		return []string{}
	}
	list, ok := idx.Children[dirPath]
	if !ok || len(list) == 0 {
		return []string{}
	}
	children := make([]string, len(list))
	copy(children, list)
	return children
}

func (idx *FileIndex) Walk(root string, walkFn func(pathStr string, info FileInfo, isDir bool) bool) {
	root = cleanPathFast(root)

	var traverse func(string) bool
	traverse = func(current string) bool {
		if info, ok := idx.GetFileInfo(current); ok {
			if !walkFn(current, info, false) {
				return false
			}
		} else if idx.HasDir(current) {
			if !walkFn(current, FileInfo{}, true) {
				return false
			}
			children := idx.GetChildren(current)
			for _, child := range children {
				childPath := path.Join(current, child)
				if !traverse(childPath) {
					return false
				}
			}
		}
		return true
	}

	traverse(root)
}
