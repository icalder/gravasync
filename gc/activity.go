package gc

import (
	"fmt"
	"time"
)

type Activity struct {
	ID         int64
	Name       string
	UploadDate time.Time
	StartTime  time.Time
	EndTime    time.Time
}

func (act Activity) String() string {
	return fmt.Sprintf("%d %s %v", act.ID, act.Name, act.UploadDate)
}
