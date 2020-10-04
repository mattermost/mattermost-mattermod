package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// StringArray type to load and save []string in mysql column as json using sqlx
type StringArray []string

// Value converts StringArray to database value
func (sa StringArray) Value() (driver.Value, error) {
	return json.Marshal(sa)
}

// Scan converts database column value to StringArray
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
