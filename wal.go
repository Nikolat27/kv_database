package main

import (
	"bufio"
	"encoding/json"
	"os"
)

type Record struct {
	Operation string
	Key       string
	Value     string
}

type WAL struct {
	file *os.File
}

func newRecord(operation, key, value string) *Record {
	return &Record{
		Operation: operation,
		Key: key,
		Value: value,
	}
}

func newWal(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &WAL{
		file: file,
	}, nil
}

func (w *WAL) Append(record *Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = w.file.Write(data)
	if err != nil {
		return err
	}

	return w.file.Sync()
}

func (w *WAL) ReadAll() ([]Record, error) {
	var records []Record

	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(w.file)
	
	for scanner.Scan() {
		var record Record

		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err 
	}

	return records, nil
}

func (w *WAL) Close() error {
	return w.file.Close()
}
