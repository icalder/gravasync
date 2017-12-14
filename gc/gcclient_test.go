package gc

import "testing"
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
