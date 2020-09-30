package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	_ "github.com/go-sql-driver/mysql" // Load MySQL Driver
)

type JSONText []string

func (j JSONText) Value() (driver.Value, error) {
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (j *JSONText) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var source []byte
	switch t := value.(type) {
	case []uint8:
		source = t
	default:
		return errors.New("could not deserialize pointer to string from db field")
	}

	return json.Unmarshal(source, j)
}
