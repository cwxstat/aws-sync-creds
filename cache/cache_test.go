package cache

import (
	"fmt"
	"os"
	"testing"
)

func TestBuildDBs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
	d, err := BuildDBs(home + "/.aws/cli/cache/")
	fmt.Println(d)

}

func TestAllProfiles(t *testing.T) {
	a, err := AllProfiles()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(a)
}

func Test_E2E(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
	d, err := BuildDBs(home + "/.aws/cli/cache/")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(d)

	path := "$HOME/.aws"
	name := "credentials"
	err = SetProfile(path, name, d.List())
	if err != nil {
		t.Error(err)
	}
}
