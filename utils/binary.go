package utils

import (
	"encoding/gob"
	"os"
)

// SaveToFile Save an object to a file using encoding/gob
func SaveToFile(filename string, obj interface{}) error {
	file, err := os.Create("/data/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(obj)
	if err != nil {
		return err
	}
	return nil
}

// ReadFromFile Read an object from a file using encoding/gob
func ReadFromFile(filename string, obj interface{}) error {
	file, err := os.Open("/data/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(obj)
	if err != nil {
		return err
	}
	return nil
}

func DeleteFile(filename string) error {
	err := os.Remove("/data/" + filename)
	if err != nil {
		return err
	}
	return nil
}
