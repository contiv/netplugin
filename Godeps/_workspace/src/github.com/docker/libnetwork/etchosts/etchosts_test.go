package etchosts

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

func TestBuildDefault(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	// check that /etc/hosts has consistent ordering
	for i := 0; i <= 5; i++ {
		err = Build(file.Name(), "", "", "", nil)
		if err != nil {
			t.Fatal(err)
		}

		content, err := ioutil.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		expected := "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\nfe00::0\tip6-localnet\nff00::0\tip6-mcastprefix\nff02::1\tip6-allnodes\nff02::2\tip6-allrouters\n"

		if expected != string(content) {
			t.Fatalf("Expected to find '%s' got '%s'", expected, content)
		}
	}
}

func TestBuildHostnameDomainname(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "10.11.12.13", "testhostname", "testdomainname", nil)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildHostname(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "10.11.12.13", "testhostname", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildNoIP(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "testhostname", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := ""; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestUpdate(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := Build(file.Name(), "10.11.12.13", "testhostname", "testdomainname", nil); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if err := Update(file.Name(), "1.1.1.1", "testhostname"); err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "1.1.1.1\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestAddEmpty(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		Record{
			Hosts: "testhostname",
			IP:    "2.2.2.2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\ttesthostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestDeleteEmpty(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		Record{
			Hosts: "testhostname1",
			IP:    "1.1.1.1",
		},
		Record{
			Hosts: "testhostname2",
			IP:    "2.2.2.2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{
		Record{
			Hosts: "testhostname1",
			IP:    "1.1.1.1",
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\ttesthostname2\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if expected := "1.1.1.1\ttesthostname1\n"; bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Did not expect to find '%s' got '%s'", expected, content)
	}
}
