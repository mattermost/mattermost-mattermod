// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SQLSpinmintStore struct {
	*SQLStore
}

func NewSQLSpinmintStore(sqlStore *SQLStore) SpinmintStore {
	s := &SQLSpinmintStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Spinmint{}, "Spinmint").SetKeys(false, "InstanceId")
		table.ColMap("InstanceId").SetMaxSize(128)
	}

	return s
}

func (s SQLSpinmintStore) CreateIndexesIfNotExists() {
	s.CreateColumnIfNotExists("Spinmint", "InstanceId", "varchar(128)", "varchar(128)", "")
}

func (s SQLSpinmintStore) Save(spinmint *model.Spinmint) Channel {
	storeChannel := make(Channel)

	go func() {
		result := Result{}

		if err := s.GetMaster().Insert(spinmint); err != nil {
			if _, err := s.GetMaster().Update(spinmint); err != nil {
				result.Err = model.NewLocAppError("SQLSpinmintStore.Save", "Could not insert or update spinmint", nil,
					fmt.Sprintf("instanceid=%v, owner=%v, name=%v, number=%v, err=%v", spinmint.InstanceID, spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, err.Error()))
			}
		}

		if result.Err == nil {
			result.Data = spinmint
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SQLSpinmintStore) List() Channel {
	storeChannel := make(Channel)

	go func() {
		result := Result{}

		var spinmint []*model.Spinmint
		if _, err := s.GetReplica().Select(&spinmint,
			`SELECT
        *
      FROM
        Spinmint`); err != nil {
			result.Err = model.NewLocAppError("SQLSpinmintStore.List", "Could not list spinmint", nil, err.Error())
		} else {
			result.Data = spinmint
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SQLSpinmintStore) Get(prNumber int, repoName string) Channel {
	storeChannel := make(Channel)

	go func() {
		result := Result{}

		var spinmint model.Spinmint
		if err := s.GetReplica().SelectOne(&spinmint,
			`SELECT * FROM
        Spinmint
      WHERE
        Number = :prNumber AND RepoName = :repoName`, map[string]interface{}{"prNumber": prNumber, "repoName": repoName}); err != nil {
			if err != sql.ErrNoRows {
				result.Err = model.NewLocAppError("SQLSpinmintStore.Get", "Could not get the spinmint", nil,
					fmt.Sprintf("owner=%v, name=%v, number=%v, instanceid=%v, err=%v", spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, spinmint.InstanceID, err.Error()))
			} else {
				result.Data = nil
			}
		} else {
			result.Data = &spinmint
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SQLSpinmintStore) Delete(instanceID string) Channel {
	storeChannel := make(Channel)
	go func() {
		result := Result{}

		var spinmint []*model.Spinmint
		if _, err := s.GetReplica().Select(&spinmint,
			`DELETE FROM
        Spinmint
      WHERE
        InstanceId = :InstanceID`, map[string]interface{}{"InstanceID": instanceID}); err != nil {
			result.Err = model.NewLocAppError("SQLSpinmintStore.Delete", "Could not list spinmint", nil, err.Error())
		} else {
			result.Data = spinmint
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}
