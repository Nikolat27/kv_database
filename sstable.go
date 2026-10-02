package main

import (
	"io"
	"os"
	"encoding/binary"
)

type SSTable struct {
	file *os.File
}

type SSTableRecord struct {
	Key string
	Value string
	Tombstone bool
}

func newSSTable(path string) (*SSTable, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &SSTable{
		file: file,
	}, nil
}

func (st *SSTable) Write(key, value string, tombstone bool) error {
	var keyLength [4]byte
	var valueLength [4]byte

	keyBytes := []byte(key)
	valueBytes := []byte(value)

	var tombstoneBytes byte

	if tombstone {
		tombstoneBytes = 1
	} else {
		tombstoneBytes = 0
	}

	binary.BigEndian.PutUint32(keyLength[:], uint32(len(keyBytes)))
	binary.BigEndian.PutUint32(valueLength[:], uint32(len(valueBytes)))

	if _, err := st.file.Write(keyLength[:]); err != nil {
		return err
	}

	if _, err := st.file.Write(valueLength[:]); err != nil {
		return err
	}

	if _, err := st.file.Write([]byte{tombstoneBytes}); err != nil {
		return err
	}

	if _, err := st.file.Write(keyBytes); err != nil {
		return err
	}

	if _, err := st.file.Write(valueBytes); err != nil {
		return err
	}

	return nil
}

func (st *SSTable) ReadAll() ([]SSTableRecord, error) {
	var records []SSTableRecord

	if _, err := st.file.Seek(0, 0); err != nil {
		return nil, err
	}

	for {
		var keyLengthBytes [4]byte
		_, err := io.ReadFull(st.file, keyLengthBytes[:])
		if err != nil {
			if err == io.EOF {
				break
			}
			
			return nil, err
		}

		keyLength := binary.BigEndian.Uint32(keyLengthBytes[:])


		var valueLengthBytes [4]byte
		_, err = io.ReadFull(st.file, valueLengthBytes[:])
		if err != nil {
			return nil, err
		}

		valueLength := binary.BigEndian.Uint32(valueLengthBytes[:])


		var tombstoneBytes [1]byte
		if _, err := io.ReadFull(st.file, tombstoneBytes[:]); err != nil {
			return nil, err
		}

		keyBytes := make([]byte, keyLength)

		if _, err := io.ReadFull(st.file, keyBytes); err != nil {
			return nil, err
		}

		valueBytes := make([]byte, valueLength)

		if _, err := io.ReadFull(st.file, valueBytes); err != nil {
			return nil, err
		}

		tombstone := tombstoneBytes[0] == 1

		record := SSTableRecord{
			Key: string(keyBytes),
			Value: string(valueBytes),
			Tombstone: tombstone,
		}

		records = append(records, record)
	}

	return records, nil
}

func (st *SSTable) Sync() error {
	return st.file.Sync()
}

func (st *SSTable) Close() error {
	return st.file.Close()
}

// Value, Existence, Tombstone, Error
func (st *SSTable) Get(key string) (string, bool, bool, error) {
	if _, err := st.file.Seek(0, 0); err != nil {
		return "", false, false, err
	}

	for {
		var keyLengthBytes [4]byte
		_, err := io.ReadFull(st.file, keyLengthBytes[:])
		if err != nil {
			if err == io.EOF {
        		return "", false, false, nil
    		}

			return "", false, false, err
		}

		keyLength := binary.BigEndian.Uint32(keyLengthBytes[:])


		var valueLengthBytes [4]byte
		_, err = io.ReadFull(st.file, valueLengthBytes[:])
		if err != nil {
			return "", false, false, err
		}
		
		valueLength := binary.BigEndian.Uint32(valueLengthBytes[:])

		var tombstoneBytes [1]byte
		if _, err := io.ReadFull(st.file, tombstoneBytes[:]); err != nil {
			return "", false, false, err
		}

		tombstone := tombstoneBytes[0] == 1

		keyBytes := make([]byte, keyLength)

		if _, err := io.ReadFull(st.file, keyBytes); err != nil {
			return "", false, tombstone, err
		}

		valueBytes := make([]byte, valueLength)

		if _, err := io.ReadFull(st.file, valueBytes); err != nil {
			return "", false, tombstone, err
		}

		if string(keyBytes) == key {
			if tombstone {
				return "", true, true, nil
			}

			return string(valueBytes), true, false, nil
		}
	}
}
