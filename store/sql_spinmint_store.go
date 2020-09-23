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

func (s SQLSpinmintStore) Save(spinmint *model.Spinmint) (*model.Spinmint, error) {
	if err := s.GetMaster().Insert(spinmint); err != nil {
		if _, err := s.GetMaster().Update(spinmint); err != nil {
			return nil, fmt.Errorf("could not insert or update spinmint: instanceid=%v, owner=%v, name=%v, number=%v, err=%w",
				spinmint.InstanceID, spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, err)
		}
	}
	return spinmint, nil
}

func (s SQLSpinmintStore) List() ([]*model.Spinmint, error) {
	var spinmints []*model.Spinmint
	_, err := s.GetReplica().Select(&spinmints,
		`SELECT
        *
      FROM
        Spinmint`)
	if err != nil {
		return nil, fmt.Errorf("could not list spinmints: %w", err)
	}
	return spinmints, nil
}

func (s SQLSpinmintStore) Get(prNumber int, repoName string) (*model.Spinmint, error) {
	var spinmint model.Spinmint
	if err := s.GetReplica().SelectOne(&spinmint,
		`SELECT * FROM
        Spinmint
      WHERE
        Number = :prNumber AND RepoName = :repoName`, map[string]interface{}{"prNumber": prNumber, "repoName": repoName}); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("could not get the spinmint: owner=%v, name=%v, number=%v, instanceid=%v, err=%w", spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, spinmint.InstanceID, err)
		}
		return nil, nil // row not found.
	}
	return &spinmint, nil
}

func (s SQLSpinmintStore) Delete(instanceID string) error {
	if _, err := s.GetReplica().Exec(`DELETE FROM
        Spinmint
      WHERE
        InstanceId = :InstanceID`, map[string]interface{}{"InstanceID": instanceID}); err != nil {
		return fmt.Errorf("could not delete spinmint: instanceid=%v, err=%w", instanceID, err)
	}
	return nil
}
