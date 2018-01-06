package strava

import (
	"fmt"
	"os"
	"testing"

	"github.com/icalder/gravasync/gc"
)

func TestTopActivity(t *testing.T) {
	strava := NewStrava()
	strava.SetAccessToken(os.Getenv("STRAVATOKEN"))
	activity, err := strava.TopActivity()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(activity)
}

func TestUploadActivity(t *testing.T) {
	garminClient := gc.NewGarminConnect(os.Getenv("GSYNCUSER"), os.Getenv("GSYNCPW"))
	if err := garminClient.Login(); err != nil {
		t.Fatal(err)
	}
	garminClient.NextActivity()
	garminClient.NextActivity()
	activity := garminClient.NextActivity()
	if activity == nil {
		t.Fatal(fmt.Errorf("activity == nil"))
	}
	fmt.Println(activity)
	tcxBytes, err := garminClient.ExportTCX(activity.ID)
	if err != nil {
		t.Fatal(err)
	}

	strava := NewStrava()
	strava.SetAccessToken(os.Getenv("STRAVATOKEN"))
	err = strava.ImportTCX(activity.Name, true, tcxBytes)
	if err != nil {
		t.Fatal(err)
	}
}
