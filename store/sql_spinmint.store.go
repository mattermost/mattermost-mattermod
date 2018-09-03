// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package store

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-mattermod/model"
)

type SqlSpinmintStore struct {
	*SqlStore
}

func NewSqlSpinmintStore(sqlStore *SqlStore) SpinmintStore {
	s := &SqlSpinmintStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Spinmint{}, "Spinmint").SetKeys(false, "InstanceId")
		table.ColMap("InstanceId").SetMaxSize(128)
	}

	return s
}

func (s SqlSpinmintStore) CreateIndexesIfNotExists() {
	s.CreateColumnIfNotExists("Spinmint", "InstanceId", "varchar(128)", "varchar(128)", "")
}

func (s SqlSpinmintStore) Save(spinmint *model.Spinmint) StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		if err := s.GetMaster().Insert(spinmint); err != nil {
			if _, err := s.GetMaster().Update(spinmint); err != nil {
				result.Err = model.NewLocAppError("SqlSpinmintStore.Save", "Could not insert or update spinmint", nil,
					fmt.Sprintf("instanceid=%v, owner=%v, name=%v, number=%v, err=%v", spinmint.InstanceId, spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, err.Error()))
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

func (s SqlSpinmintStore) List() StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var spinmint []*model.Spinmint
		if _, err := s.GetReplica().Select(&spinmint,
			`SELECT
        *
      FROM
        Spinmint`); err != nil {
			result.Err = model.NewLocAppError("SqlSpinmintStore.List", "Could not list spinmint", nil, err.Error())
		} else {
			result.Data = spinmint
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}

func (s SqlSpinmintStore) Get(prNumber int) StoreChannel {
	storeChannel := make(StoreChannel)

	go func() {
		result := StoreResult{}

		var spinmint model.Spinmint
		if err := s.GetReplica().SelectOne(&spinmint,
			`SELECT * FROM
        Spinmint
      WHERE
        Number = :prNumber`, map[string]interface{}{"prNumber": prNumber}); err != nil {
			if err != sql.ErrNoRows {
				result.Err = model.NewLocAppError("SqlSpinmintStore.Get", "Could not get the spinmint", nil,
					fmt.Sprintf("owner=%v, name=%v, number=%v, instanceid=%v, err=%v", spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, spinmint.InstanceId, err.Error()))
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

func (s SqlSpinmintStore) Delete(instanceid string) StoreChannel {
	storeChannel := make(StoreChannel)
	go func() {
		result := StoreResult{}

		var spinmint []*model.Spinmint
		if _, err := s.GetReplica().Select(&spinmint,
			`DELETE FROM
        Spinmint
      WHERE
        InstanceId = :InstanceId`, map[string]interface{}{"InstanceId": instanceid}); err != nil {
			result.Err = model.NewLocAppError("SqlSpinmintStore.Delete", "Could not list spinmint", nil, err.Error())
		} else {
			result.Data = spinmint
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}
