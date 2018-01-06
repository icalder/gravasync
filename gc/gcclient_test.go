package gc

import (
	"io/ioutil"
	"testing"
)
import "fmt"
import "os"

func TestLogin(t *testing.T) {
	gc := NewGarminConnect(os.Getenv("GSYNCUSER"), os.Getenv("GSYNCPW"))
	if err := gc.Login(); err != nil {
		t.Fatal(err)
	}
}

func TestGetActivities(t *testing.T) {
	gc := NewGarminConnect(os.Getenv("GSYNCUSER"), os.Getenv("GSYNCPW"))
	if err := gc.Login(); err != nil {
		t.Fatal(err)
	}
	activity := gc.NextActivity()
	if activity == nil {
		t.Fatal(fmt.Errorf("activity == nil"))
	}
	fmt.Println(activity)

	activity = gc.NextActivity()
	if activity == nil {
		t.Fatal(fmt.Errorf("activity == nil"))
	}
	fmt.Println(activity)
}

func TestExportActivity(t *testing.T) {
	gc := NewGarminConnect(os.Getenv("GSYNCUSER"), os.Getenv("GSYNCPW"))
	if err := gc.Login(); err != nil {
		t.Fatal(err)
	}
	activity := gc.NextActivity()
	if activity == nil {
		t.Fatal(fmt.Errorf("activity == nil"))
	}
	fmt.Println(activity)
	tcxBytes, err := gc.ExportTCX(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	ioutil.WriteFile("/tmp/activity.tcx", tcxBytes, 0644)
}
