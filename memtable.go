package main

import (
	"sort"
)

const mt_THRESHOLD = 1_000_000 // 1MB

type MemTable struct {
	data map[string]MemTableEntry
	size int
	threshold int
}

type MemTableEntry struct {
	Value string
	Tombstone bool
}

func newMemTable() *MemTable {
	return &MemTable{
		data: make(map[string]MemTableEntry),
		size: 0,
		threshold: mt_THRESHOLD,
	}
}

func (mt *MemTable) Set(key, value string) {
	size := mt.sizeDelta(key, value)

	mt.size += size

	mt.data[key] = MemTableEntry{
		Value: value,
		Tombstone: false,
	}
}

// Value, Exists, Tombstone
func (mt *MemTable) Get(key string) (string, bool, bool) {
	data, ok := mt.data[key]
	if !ok {
		return "", false, false
	}

	if data.Tombstone {
		return "", false, true
	}

	return data.Value, true, false
}

func (mt *MemTable) Delete(key string) {
	oldEntry, exists := mt.data[key]
	if !exists {
		mt.size += len(key)

		mt.data[key] = MemTableEntry{
			Tombstone: true,
		}

		return
	}

	oldSize := len(key) + len(oldEntry.Value)
	newSize := len(key)

	mt.size += newSize - oldSize

	mt.data[key] = MemTableEntry{
		Tombstone: true,
	}
}

// WillGetFull -> This function says that will the MemTable gets full If we set(insert) the new key value or not
func (mt *MemTable) WillGetFull(key, currentValue string) bool {
	currentSize := mt.size

	size := mt.sizeDelta(key, currentValue)

	newSize := currentSize + size

	if newSize > mt.threshold {
		return true
	} else {
		return false
	}
}

// sizeDelta -> The new size that is going to get inserted
func (mt *MemTable) sizeDelta(key, currentValue string) int {
	// does the key exist or not
	data, exist := mt.data[key]

	oldValue := data.Value

	var size int
	if !exist {
		size = len(key) + len(currentValue)
	} else {
		size -= len(oldValue)
		size += len(currentValue)
	}

	return size
}

func (mt *MemTable) Flush(path string) error {
	sortedKeys := sortByKeys(mt.data)
	
	ssTable, err := newSSTable(path)
	if err != nil {
		return err
	}

	for _, key := range sortedKeys {
		data := mt.data[key]

		if err := ssTable.Write(key, data.Value, data.Tombstone); err != nil {
			ssTable.Close()
			return err
		}
	}

	if err := ssTable.Sync(); err != nil {
		ssTable.Close()
		return err
	}

	if err := ssTable.Close(); err != nil {
		return err
	}

	mt.data = make(map[string]MemTableEntry)
	mt.size = 0

	return nil
}

func sortByKeys(dataMap map[string]MemTableEntry) []string {
	newKeys := make([]string, 0, len(dataMap))

	for k := range dataMap {
		newKeys = append(newKeys, k)
	}

	sort.Strings(newKeys)

	return newKeys
}
