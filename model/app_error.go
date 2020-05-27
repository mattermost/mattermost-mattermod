// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"
)

type AppError struct {
	ID            string                 `json:"id"`
	Message       string                 `json:"message"`               // Message to be display to the end user without debugging information
	DetailedError string                 `json:"detailed_error"`        // Internal error string to help the developer
	RequestID     string                 `json:"request_id,omitempty"`  // The RequestID that's also set in the header
	StatusCode    int                    `json:"status_code,omitempty"` // The http status code
	Where         string                 `json:"-"`                     // The function where it happened in the form of Struct.Func
	IsOAuth       bool                   `json:"is_oauth,omitempty"`    // Whether the error is OAuth specific
	Params        map[string]interface{} `json:"-"`
}

func (er *AppError) Error() string {
	return er.Where + ": " + er.Message + ", " + er.DetailedError
}

func (er *AppError) ToJSON() string {
	b, err := json.Marshal(er)
	if err != nil {
		return ""
	}

	return string(b)
}

// AppErrorFromJson will decode the input and return an AppError
func AppErrorFromJSON(data io.Reader) *AppError {
	str := ""
	bytes, rerr := ioutil.ReadAll(data)
	if rerr != nil {
		str = rerr.Error()
	} else {
		str = string(bytes)
	}

	decoder := json.NewDecoder(strings.NewReader(str))
	var er AppError
	err := decoder.Decode(&er)
	if err != nil {
		return NewLocAppError("AppErrorFromJson", "model.utils.decode_json.app_error", nil, "body: "+str)
	}

	return &er
}

func NewAppError(where string, id string, params map[string]interface{}, details string, status int) *AppError {
	ap := &AppError{}
	ap.ID = id
	ap.Params = params
	ap.Message = id
	ap.Where = where
	ap.DetailedError = details
	ap.StatusCode = status
	ap.IsOAuth = false
	return ap
}

func NewLocAppError(where string, id string, params map[string]interface{}, details string) *AppError {
	ap := &AppError{}
	ap.ID = id
	ap.Params = params
	ap.Message = id
	ap.Where = where
	ap.DetailedError = details
	ap.StatusCode = 500
	ap.IsOAuth = false
	return ap
}
