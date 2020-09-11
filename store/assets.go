//go:generate go-bindata -nometadata -mode 0644 -pkg migrations -o ./migrations/bindata.go -prefix "migrations/" -ignore bindata.go migrations
// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.
package store
