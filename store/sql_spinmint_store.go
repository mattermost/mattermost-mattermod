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
	return &SQLSpinmintStore{sqlStore}
}

func (s SQLSpinmintStore) Save(spinmint *model.Spinmint) (*model.Spinmint, error) {
	insertQuery := `INSERT INTO Spinmint (InstanceId, RepoOwner, RepoName, Number, CreatedAt)
        VALUES (:InstanceId, :RepoOwner, :RepoName, :Number, :CreatedAt)`
	if _, err := s.dbx.NamedExec(insertQuery, spinmint); err != nil {
		updateQuery := `UPDATE Spinmint
			SET RepoOwner = :RepoOwner, RepoName = :RepoName, Number = :Number, CreatedAt = :CreatedAt
			WHERE InstanceId = :InstanceId`
		if _, err := s.dbx.NamedExec(updateQuery, spinmint); err != nil {
			return nil, fmt.Errorf("could not insert or update spinmint: instanceid=%v, owner=%v, name=%v, number=%v, err=%w",
				spinmint.InstanceID, spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, err)
		}
	}
	return spinmint, nil
}

func (s SQLSpinmintStore) List() ([]*model.Spinmint, error) {
	spinmints := []*model.Spinmint{}
	err := s.dbx.Select(&spinmints,
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
	if err := s.dbx.Get(&spinmint,
		`SELECT * FROM
        Spinmint
      WHERE
        Number = ? AND RepoName = ?`, prNumber, repoName); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("could not get the spinmint: owner=%v, name=%v, number=%v, instanceid=%v, err=%w", spinmint.RepoOwner, spinmint.RepoName, spinmint.Number, spinmint.InstanceID, err)
		}
		return nil, nil // row not found.
	}
	return &spinmint, nil
}

func (s SQLSpinmintStore) Delete(instanceID string) error {
	if _, err := s.dbx.NamedExec(`DELETE FROM
        Spinmint
      WHERE
        InstanceId = :InstanceID`, map[string]interface{}{"InstanceID": instanceID}); err != nil {
		return fmt.Errorf("could not delete spinmint: instanceid=%v, err=%w", instanceID, err)
	}
	return nil
}
