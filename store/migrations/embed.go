// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package migrations

import "embed"

// FS holds the SQL migrations, embedded at compile time.
//
//go:embed *.sql
var FS embed.FS
