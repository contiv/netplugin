package modeldb

import (
	"errors"
	"testing"

	log "github.com/Sirupsen/logrus"
)

type dummyModel struct {
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

func (obj *dummyModel) GetType() string {
	return "dummyModel"
}

func (obj *dummyModel) GetKey() string {
	return obj.Key
}

func (obj *dummyModel) Read() error {
	if obj.Key == "" {
		log.Errorf("Empty key while trying to read app object")
		return errors.New("Empty key")
	}

	return ReadObj("dummyModel", obj.Key, obj)
}

func (obj *dummyModel) Write() error {
	if obj.Key == "" {
		log.Errorf("Empty key while trying to Write app object")
		return errors.New("Empty key")
	}

	return WriteObj("dummyModel", obj.Key, obj)
}

func TestSetGetDel(t *testing.T) {
	var testObj = dummyModel{Key: "testKey", Name: "testName"}
	var readObj = dummyModel{Key: "testKey"}

	// test write
	err := testObj.Write()
	if err != nil {
		t.Fatalf("Error writing test object. Err: %v", err)
	}

	// read object
	err = readObj.Read()
	if err != nil {
		t.Fatalf("Error reading test object. Err: %v", err)
	}

	// verify we read valid object
	if readObj.Name != testObj.Name {
		t.Fatalf("Read value (%s) does not match written value (%s)", readObj.Name, testObj.Name)
	}

	// tets delete
	err = DeleteObj(testObj.GetType(), testObj.Key)
	if err != nil {
		t.Fatalf("Error deleting test object. Err: %v", err)
	}
}
