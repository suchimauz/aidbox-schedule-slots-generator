package json_types

import (
	"encoding/json"
	"fmt"
	"time"
)

type Time struct {
	Time time.Time
}

func (t *Time) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])
	parsedTime, err := time.Parse("15:04:05", str)
	if err != nil {
		return fmt.Errorf("failed to parse time: %v", err)
	}
	*t = Time{Time: parsedTime}
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.Format("15:04:05"))
}
