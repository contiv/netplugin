package modeldb

import (
	"errors"
	"os"
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

func TestMain(m *testing.M) {
	// init modeldb
	Init("")

	os.Exit(m.Run())
}

func testSetGetDel(t *testing.T, dbURL string) {
	var testObj = dummyModel{Key: "testKey", Name: "testName"}
	var readObj = dummyModel{Key: "testKey"}

	// init modeldb
	Init(dbURL)

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

func TestSetGetDel(t *testing.T) {
	testSetGetDel(t, "")
}

func TestSetGetDelEtcd(t *testing.T) {
	testSetGetDel(t, "etcd://localhost:2379")
}

func TestSetGetDelConsul(t *testing.T) {
	testSetGetDel(t, "consul://localhost:8500")
}
