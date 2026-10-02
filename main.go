package main

import (
	"fmt"
	"log"
)

func main() {
	engine, err := newEngine(newMemTable())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer engine.Close()

	engine.Set("name", "sam")
	engine.Set("name2", "john")
	engine.Set("name3", "alice")

	engine.Delete("name2")

	engine.Set("name", "bob")
	engine.Set("name4", "david")
	engine.Set("name5", "mike")
	engine.Set("name6", "tom")
	engine.Set("name7", "jane")
	engine.Delete("name7")

	val, exist, tombstone, err := engine.Get("name7")
	if err != nil {
		log.Fatal(err)
	}

	log.Println(tombstone)
	log.Println(exist)
	log.Println(val)

	val, exist, tombstone, err = engine.Get("name")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("name:")
	log.Println("value:", val)
	log.Println("exists:", exist)
	log.Println("tombstone:", tombstone)
}
