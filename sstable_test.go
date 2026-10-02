package main

import (
	"reflect"
	"testing"
)

func TestSSTablePersistence(t *testing.T) {
	newMt := newMemTable()

	newMt.Set("name", "sam")
	newMt.Set("age", "20")
	newMt.Set("gender", "male")

	path := "data/test.sst"

	if err := newMt.Flush(path); err != nil {
		t.Fatal(err)
	}

	sstable, err := newSSTable(path)
	if err != nil {
		t.Fatal(err)
	}

	records, err := sstable.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	// we should make these ordered
	expected := []SSTableRecord{
		{Key: "age", Value: "20"},
		{Key: "gender", Value: "male"},
		{Key: "name", Value: "sam"},
	}

	if !reflect.DeepEqual(records, expected) {
		t.Fatalf("expected %v, got %v", expected, records)
	}

	t.Log("Everything works fine")
}
