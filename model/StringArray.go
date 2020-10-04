package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	_ "github.com/go-sql-driver/mysql" // Load MySQL Driver
)

type StringArray []string

func (sa StringArray) Value() (driver.Value, error) {
	b, err := json.Marshal(sa)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (sa *StringArray) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	buf, ok := value.([]byte)
	if !ok {
		return errors.New("received value is not a byte slice")
	}

	return json.Unmarshal(buf, sa)
}
