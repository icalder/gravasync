package strava

import (
	"fmt"
	"time"
)

type Activity struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"start_date"`
}

func (act Activity) String() string {
	return fmt.Sprintf("%d %s %v", act.ID, act.Name, act.StartDate)
}
