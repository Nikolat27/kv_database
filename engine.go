package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const COMPACTION_THRESHOLD = 4

type Engine struct {
	MemTable *MemTable

	wal      *WAL
	walPath  string

	ssTablePaths []string

	nextSSTableID uint64
	nextWALID uint64
}

func newEngine(mp *MemTable) (*Engine, error) {
	e := &Engine{
		MemTable: mp,
	}

	walPaths, err := e.getAllWalPaths()
	if err != nil {
		return nil, err
	}

	e.nextWALID = getNextID(walPaths)

	var currentWalPath string
	if len(walPaths) > 0 {
		currentWalPath = walPaths[len(walPaths)-1]
	} else {
		currentWalPath = e.nextWALPath()
	}

	newWal, err := newWal(currentWalPath)
	if err != nil {
		return nil, err
	}

	e.wal = newWal
	e.walPath = currentWalPath

	if err := e.ReplayAllWals(); err != nil {
		return nil, err
	}

	paths, err := e.getAllssTablePaths()
	if err != nil {
		return nil, err
	}

	e.ssTablePaths = paths
	e.nextSSTableID = getNextID(paths)

	return e, nil
}

func (e *Engine) Set(key, value string) error {
	willGetFull := e.MemTable.WillGetFull(key, value)

	if willGetFull {
		path := e.nextSSTablePath()

		if err := e.MemTable.Flush(path); err != nil {
			return err
		}

		e.ssTablePaths = append(e.ssTablePaths, path)

		if err := e.rotateWAL(); err != nil {
			return err
		}

		if len(e.ssTablePaths) >= COMPACTION_THRESHOLD {
			if err := e.Compact(); err != nil {
				return err
			}
		}
	}

	newRecord := newRecord("SET", key, value)
	if err := e.wal.Append(newRecord); err != nil {
		return err
	}

	e.MemTable.Set(key, value)
	return nil
}

// Value, Existence, Tombstone, Error
func (e *Engine) Get(key string) (string, bool, bool, error) {
	value, exist, tombstone := e.MemTable.Get(key)
	if exist {
		return value, true, false, nil
	}

	if tombstone {
		return "", false, true, nil
	}

	value, exist, tombstone, err := e.getFromSSTable(key)
	if err != nil {
		return "", false, false, err
	}
	
	if tombstone {
		return "", false, true, nil
	}

	if !exist {
		return "", false, false, nil
	}

	return value, true, false, nil
}

func (e *Engine) Delete(key string) error {
	newRecord := newRecord("DELETE", key, "")

	if err := e.wal.Append(newRecord); err != nil {
		return err
	}

	e.MemTable.Delete(key)
	return nil
}

func (e *Engine) Apply(record *Record) {
	e.MemTable.Set(record.Key, record.Value)
}

func (e *Engine) ReplayAllWals() error {
	allWalPaths, err := e.getAllWalPaths()
	if err != nil {
		return err
	}

	for _, walPath := range allWalPaths {
		wal, err := newWal(walPath)
		if err != nil {
			return err
		}

		if err := e.Replay(wal); err != nil {
			wal.Close()
			return err
		}

		if err := wal.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) Replay(wal *WAL) error {
	records, err := wal.ReadAll()
	if err != nil {
		return err
	}

	for _, record := range records {
		switch record.Operation {
		case "SET":
			e.Apply(&record)
		case "DELETE":
			e.MemTable.Delete(record.Key)
		default:
			return fmt.Errorf("unknown operation: %s", record.Operation)
		}
	}

	return nil
}

func (e *Engine) nextSSTablePath() string {
	path := fmt.Sprintf("data/%06d.sst", e.nextSSTableID)
	e.nextSSTableID++

	return path
}

func (e *Engine) getAllssTablePaths() ([]string, error) {
	var ssTablePaths []string

	entries, err := os.ReadDir("data")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".sst" {
			continue
		}

		path := filepath.Join("data", entry.Name())

		ssTablePaths = append(ssTablePaths, path)
	}

	sort.Strings(ssTablePaths)

	return ssTablePaths, nil
}

// Value, Existence, Tombstone, Error
func (e *Engine) getFromSSTable(key string) (string, bool, bool, error) {
	paths := e.ssTablePaths

	if paths == nil {
		return "", false, false, nil
	}

	// we have to iterate backwards for checking the newest paths first
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]

		ssTable, err := newSSTable(path)
		if err != nil {
			return "", false, false, err
		}
		defer ssTable.Close()

		value, exist, tombstone, err := ssTable.Get(key)
		if err != nil {
			return "", false, false, err
		}

		if tombstone {
			return "", false, true, nil
		}

		if !exist {
			continue
		}

		return value, true, false, nil
	}

	return "", false, false, nil
}

func (e *Engine) nextWALPath() string {
	path := fmt.Sprintf("data/%06d.wal", e.nextWALID)
	e.nextWALID++

	return path
}

func (e *Engine) rotateWAL() error {
	oldWalPath := e.walPath

	if err := e.wal.Close(); err != nil {
		return err
	}

	newPath := e.nextWALPath()

	newWal, err := newWal(newPath)
	if err != nil {
		return err
	}

	e.wal = newWal
	e.walPath = newPath

	if err := os.Remove(oldWalPath); err != nil {
		return err
	}

	return nil
}

func (e *Engine) getAllWalPaths() ([]string, error) {
	var walPaths []string

	entries, err := os.ReadDir("data")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".wal" {
			continue
		}

		path := filepath.Join("data", entry.Name())

		walPaths = append(walPaths, path)
	}

	sort.Strings(walPaths)

	return walPaths, nil
}

func getNextID(paths []string) uint64 {
	var maxID uint64

	for _, path := range paths {
		filename := filepath.Base(path)

		// remove extension
		idString := strings.TrimSuffix(filename, filepath.Ext(filename))

		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			continue
		}

		if id > maxID {
			maxID = id
		}
	}

	return maxID + 1
}

func (e *Engine) Close() error {
	if e.wal == nil {
		return nil
	}

	return e.wal.Close()
}

func (e *Engine) Compact() error {
	if len(e.ssTablePaths) < 2 {
		return nil
	}

	latest := make(map[string]SSTableRecord)

	for _, path := range e.ssTablePaths {
		sst, err := newSSTable(path)
		if err != nil {
			return err
		}

		records, err := sst.ReadAll()
		if err != nil {
			sst.Close()
			return err
		}

		if err := sst.Close(); err != nil {
			return err
		}

		for _, record := range records {
			latest[record.Key] = record
		}
	}

	newPath := e.nextSSTablePath()

	sst, err := newSSTable(newPath)
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(latest))

	for key := range latest {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		record := latest[key]

		if err := sst.Write(
			record.Key,
			record.Value,
			record.Tombstone,
		); err != nil  {
			sst.Close()
			return err
		}
	}

	if err := sst.Sync(); err != nil {
		sst.Close()
		return err
	}

	if err := sst.Close(); err != nil {
		return err
	}

	oldPaths := e.ssTablePaths
	e.ssTablePaths = []string{newPath}

	for _, path := range oldPaths {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}
