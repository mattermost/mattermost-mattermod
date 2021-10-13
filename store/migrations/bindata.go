// Code generated by go-bindata. DO NOT EDIT.
// sources:
// migrations/000001_base.down.sql (118B)
// migrations/000001_base.up.sql (3.007kB)
// migrations/000002_add_milestone.down.sql (958B)
// migrations/000002_add_milestone.up.sql (1.069kB)
// migrations/000003_drop_spinmint_table.down.sql (332B)
// migrations/000003_drop_spinmint_table.up.sql (49B)

package migrations

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("read %q: %w", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes  []byte
	info   os.FileInfo
	digest [sha256.Size]byte
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var __000001_baseDownSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x72\x72\x75\xf7\xf4\xb3\xe6\xe2\x72\x09\xf2\x0f\x50\x08\x71\x74\xf2\x71\x55\xf0\x74\x53\x70\x8d\xf0\x0c\x0e\x09\x56\x48\xf0\x2c\x2e\x2e\x4d\x2d\x4e\xb0\xc6\x21\x1d\x50\x9a\x93\x13\x94\x5a\x58\x9a\x5a\x5c\x82\x5b\x51\x70\x41\x66\x5e\x6e\x66\x5e\x49\x82\x35\x17\x97\xb3\xbf\xaf\xaf\x67\x88\x35\x17\x20\x00\x00\xff\xff\x9d\x30\xa8\xa4\x76\x00\x00\x00")

func _000001_baseDownSqlBytes() ([]byte, error) {
	return bindataRead(
		__000001_baseDownSql,
		"000001_base.down.sql",
	)
}

func _000001_baseDownSql() (*asset, error) {
	bytes, err := _000001_baseDownSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000001_base.down.sql", size: 0, mode: os.FileMode(0644), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xf1, 0xd4, 0xdf, 0x48, 0x25, 0xe8, 0x42, 0x8d, 0x68, 0xb8, 0xb1, 0x81, 0x57, 0xd8, 0xb, 0xb6, 0xfb, 0xac, 0x17, 0xd6, 0xe1, 0xc4, 0xfd, 0xb, 0xee, 0x4, 0xb8, 0xb6, 0x1, 0xe9, 0x43, 0xc0}}
	return a, nil
}

var __000001_baseUpSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xd4\x56\x41\x6f\xda\x30\x18\xbd\xf3\x2b\xbc\x13\xb4\x03\x35\x69\xa9\x54\xad\x42\x22\x04\xd3\x5a\x4d\xec\x36\x71\xa6\xb5\x17\x63\xc0\xb4\xd1\x82\xe9\x12\xa7\x5b\xff\xfd\x14\x28\x24\x21\x4e\xe1\x50\x4d\x1d\x27\xf3\xf9\xf9\xe5\xfb\xec\xf7\xa4\x37\x80\x57\x08\x5f\x36\x1a\x27\xc7\x5f\xba\x86\x69\x98\xc0\x87\x14\xf4\x89\x33\x64\xf6\xb5\xe5\x59\x36\x85\x1e\xf3\x21\x65\xb6\x83\x20\xa6\xbd\x7e\x5f\x57\x06\xc7\x27\x97\x7b\x19\x3c\xe8\x07\x0e\xf5\x2b\x14\x6f\xf5\x3a\x0e\xe2\x38\x16\x45\x04\x33\x9b\x60\x0c\xed\x6c\x99\x51\x68\xca\x55\x06\x6c\xb9\xd0\x07\xa9\x9a\x5f\x14\xf7\xce\x72\x76\x8a\x5c\xc8\x1e\x08\x86\xbd\x7e\x7f\xbb\xae\x62\x73\x58\xf3\xab\x61\x7c\x33\x8c\x66\x8e\x31\xcc\x6e\xce\x17\x60\x74\x17\x40\x66\x5f\x43\xfb\x26\x9b\xb4\xf4\xbf\x0d\xca\xdb\x46\x0d\xc9\x88\x78\x10\x5d\x61\x76\x03\xef\x73\xa6\x6a\xb1\x0d\x34\x40\xa3\xe6\x1a\xfd\x3b\x87\xb9\x64\x98\xcd\xb9\x59\xb6\xc1\xb6\xd8\xc4\x84\x59\x01\x25\xec\xbb\xe5\x04\x90\x11\xcc\x1e\xa0\x47\x0a\x43\x9a\xe6\x0e\x17\x26\x14\xfa\x6f\x64\xab\xf5\x9a\x6d\x5d\x5e\x37\xd1\xe8\x74\x1a\x9d\x0e\xa0\x7c\x12\x09\x90\xa8\x38\x9d\xaa\x34\x16\x60\xbe\x8c\x81\x5a\xd5\xc6\x28\x49\x52\x91\x8c\x33\xe0\x4e\xcb\x09\x7f\x11\x33\x36\x4d\xd8\x34\x0a\x85\x54\x20\xfb\xf5\x40\xbf\x3f\x7d\xe2\x31\x9f\x2a\x11\xb3\x44\xa8\xcd\x66\x65\x62\x2d\xaa\x97\xeb\xc0\xf6\xa0\x45\x21\xa0\xd6\xc0\x81\x00\x8d\x00\x26\x14\xc0\x1f\xc8\xa7\xfe\xb6\x27\xd0\x6a\x00\x30\xf6\xc4\xf3\x92\xfc\x96\x22\x1e\x83\x17\x1e\x67\xb4\x2d\xf3\xf4\xe2\x68\x75\x00\x07\x8e\xd3\xde\x80\x30\x5f\x88\xf7\x30\x38\x5d\x4c\x32\x96\x50\xaa\x96\x69\xee\x6c\x06\x89\x88\x65\x95\x60\x08\x47\x56\xe0\x14\x70\xbe\xe2\xaa\x00\xd2\x41\x1c\x3e\x11\x51\x32\x06\x4a\xfc\x51\x59\xe1\xd6\x43\xae\xe5\xdd\x83\x1b\x78\x0f\x5a\x85\x71\xda\x79\xd7\xed\x4d\x73\x47\x8d\x23\x00\xf1\x15\xc2\xb0\x87\xa4\x5c\x0e\x07\x5b\xfa\xcc\xaf\x3e\xa4\xbd\xec\x02\x17\x93\xee\x61\xb7\x5d\x79\xc3\xc3\x34\x71\x9b\x46\x91\x27\x7e\xa5\x22\x51\x9f\x4c\x19\xa5\xce\x3e\x58\x1f\xa3\x34\x8a\xca\x98\x53\xe3\xe2\x2c\x7f\xe0\x66\xf3\xe3\x64\xe4\x89\xf9\x5e\xa5\x3d\xf1\x1c\xd2\x3d\x44\x68\x87\x88\x73\x90\x86\xd1\x2c\xc3\xa5\xc9\x01\x40\x7b\x29\xa7\x51\x9a\x84\x4b\x59\xbc\x94\x3a\xb4\x13\xca\x9f\x85\x6e\x02\xcf\xd9\x73\x95\x76\x2c\xb8\x12\x33\x4b\x8d\x81\x0a\x17\x22\x51\x7c\xf1\xbc\xe2\xac\x7e\xc0\xe5\xa1\x54\x3c\x94\x22\xb6\xb9\x74\x97\xb3\x70\xfe\x9a\x1d\x92\xaf\xab\x67\xd0\x74\xe4\x8a\xf8\x51\xcc\xde\xc5\xfc\x7f\xc6\xf4\x9f\x43\xb9\x08\xa5\xfa\x5c\xa6\xdc\x76\xb5\x36\x24\x92\x89\xe2\x72\x2a\xd0\x6c\x9f\x23\x77\x6c\x7b\x7a\x7e\xae\xb5\xca\xae\x75\xf5\xb8\x5d\x5f\x56\x00\x05\xb5\x4d\xc2\xc7\x0c\xa6\x93\x72\x59\x14\x85\x59\xfe\x85\x02\x6a\x02\x4f\x39\x26\xad\xb5\x52\xfa\x46\x9e\x2c\x8a\x39\xa3\x1a\x6d\x74\xa9\x46\x9f\x76\xaa\x67\x77\x62\x55\x25\x69\x55\x35\xa5\x0f\xaf\x75\xb1\x76\xdf\xf9\x6d\x74\xad\x4d\xb5\x1a\x06\x6d\x70\xad\x8b\xb4\xd5\xa8\x55\x48\x59\xa5\xd0\xb5\x7e\x02\x9b\xb8\x2e\xa2\x97\x8d\xbf\x01\x00\x00\xff\xff\x3e\x87\xf5\x95\xbf\x0b\x00\x00")

func _000001_baseUpSqlBytes() ([]byte, error) {
	return bindataRead(
		__000001_baseUpSql,
		"000001_base.up.sql",
	)
}

func _000001_baseUpSql() (*asset, error) {
	bytes, err := _000001_baseUpSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000001_base.up.sql", size: 0, mode: os.FileMode(0644), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xd5, 0x66, 0xbb, 0x40, 0x9d, 0x1f, 0x8c, 0x17, 0x19, 0x30, 0x9b, 0xc7, 0xea, 0x76, 0xa1, 0xab, 0xf5, 0x96, 0x93, 0xf4, 0xce, 0x4b, 0x62, 0x3, 0xd3, 0xab, 0x3d, 0xd8, 0x40, 0x67, 0x91, 0xec}}
	return a, nil
}

var __000002_add_milestoneDownSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x91\xcb\x6a\xf3\x30\x10\x85\xf7\x7a\x8a\x41\x2b\xfb\xc7\xfc\xb4\x6b\x93\x52\x45\x9e\x34\x06\x5b\x32\xb2\x42\xbb\x0b\x4e\x32\xa5\x01\xdb\x49\x6d\x19\xfa\xf8\xc5\xb7\xa6\xd7\x7d\x17\x5d\x08\xc4\xcc\xa7\xa3\x39\x73\x96\x78\x17\xab\x90\xb1\x1c\x2d\xdc\x1e\x76\xaa\xa8\x08\x16\x10\x09\x2b\x96\x22\x47\xcf\x0f\xc7\x8e\x2b\x76\x25\x4d\x4d\x9e\x75\x65\x69\xe8\xb9\xa3\xd6\xb5\x7c\x02\xf6\xa7\xb2\xab\xea\x99\x48\x8f\x25\xb5\xee\x54\x93\xea\xaa\x1d\x35\x33\x74\x6e\xe8\x5c\x34\x74\xc8\x5d\xe1\xa8\xa2\xda\xc1\x02\xbc\x1c\x13\x94\x16\xe2\x95\xc7\x00\xfa\x03\x30\x95\xa4\xde\x28\xeb\xfd\xf3\x61\x65\x74\x0a\xb1\x5a\x69\x93\x0a\x1b\x6b\xb5\xcd\xe5\x1a\x53\xf1\x5f\xea\x64\x93\xaa\x7c\x78\x73\xbf\x46\x83\xc3\x0d\xc0\x1b\xc6\xdd\xd6\xe3\x34\x97\xe1\xfd\xa9\x2f\x54\x34\x33\xed\xfe\x89\xaa\xa2\xa7\x46\xf3\x1f\x90\xd1\xd4\x9b\xce\xc5\x63\x4f\xf9\x70\x03\x57\x01\x03\x90\x5a\x49\x61\x3d\x2e\x12\x8b\x06\xac\x58\x26\x08\x3c\x78\xf7\x6d\x00\x1c\x22\xa3\xb3\xa1\x7a\x11\x09\x80\x87\xdc\xef\x15\xf8\x64\xf8\x9a\x33\xdf\x0f\x59\x66\x30\x13\x06\xa1\x28\x1d\x35\xf1\x23\xbe\x1c\x5b\xd7\x8e\x4b\xf8\xba\xc2\x90\xe1\x03\xca\x8d\xfd\x84\x87\x8c\x45\x28\x92\x44\x4b\x61\x11\xbe\x55\x9c\x53\xff\x21\x3a\x7b\x74\x25\xfd\x25\xf7\x3b\x93\x93\x3a\x4d\x63\x1b\xbe\x06\x00\x00\xff\xff\x95\xe7\xea\x19\xbe\x03\x00\x00")

func _000002_add_milestoneDownSqlBytes() ([]byte, error) {
	return bindataRead(
		__000002_add_milestoneDownSql,
		"000002_add_milestone.down.sql",
	)
}

func _000002_add_milestoneDownSql() (*asset, error) {
	bytes, err := _000002_add_milestoneDownSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000002_add_milestone.down.sql", size: 0, mode: os.FileMode(0644), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x68, 0x6, 0x24, 0x4c, 0xf8, 0xa2, 0x29, 0x6c, 0x4c, 0x49, 0xb9, 0xf4, 0x35, 0xda, 0x65, 0x86, 0xf5, 0x6b, 0xc5, 0x25, 0x51, 0xf9, 0x10, 0x9d, 0x28, 0x26, 0x2d, 0xc6, 0x91, 0xb1, 0xa2, 0x2c}}
	return a, nil
}

var __000002_add_milestoneUpSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x92\xcf\x6a\xe3\x30\x10\xc6\xef\x7a\x8a\x41\x27\x6b\x31\xcb\xee\x42\x4e\x26\xcb\x2a\xf2\x64\x63\xb0\xa5\x20\x2b\x6d\x6f\xc1\x49\x54\x1a\xb0\x9d\xd4\x96\xa1\x7d\xfb\xe2\x7f\x75\x43\x9a\x07\x28\xf4\x60\xb0\x67\x7e\x33\xcc\xf7\x7d\x5e\xe0\xff\x48\x06\x84\xa4\x68\xe0\xdf\x61\x27\xb3\xc2\xc2\x1c\x42\x6e\xf8\x82\xa7\xe8\xb1\xa0\xef\xb8\x6c\x97\xdb\xa1\x49\xd7\x4d\x9e\x6b\xfb\xdc\xd8\xda\xd5\x74\x00\xf6\xa7\xbc\x29\xca\x91\x48\x8e\xb9\xad\xdd\xa9\xb4\xb2\x29\x76\xb6\xba\x84\xcc\xeb\xb9\x83\x22\x69\xc6\xc6\xb9\xb2\xe7\xac\xb2\x87\xd4\x65\xce\x16\xb6\x74\x30\x07\x2f\xc5\x18\x85\x81\x68\xe9\x11\x80\xf6\x01\x18\x4a\x42\x6d\xa4\xf1\x7e\x30\x58\x6a\x95\x40\x24\x97\x4a\x27\xdc\x44\x4a\x6e\x53\xb1\xc2\x84\xff\x14\x2a\xde\x24\x32\xed\x66\xee\x57\xa8\xb1\x7b\x03\xf0\x3a\x1d\xdb\xb2\x3f\x73\x52\xc5\x86\x3e\x97\xe1\xc8\xd4\xfb\x27\x5b\x64\x2d\xd5\xbb\x72\x81\xf4\x42\xde\xf7\x4c\xe2\x5b\x8a\xc1\x5f\xf8\xe5\x13\x00\x3a\x9c\xfb\x9b\xb6\x5f\x42\x49\xc1\x8d\x47\x79\x6c\x50\x83\xe1\x8b\x18\x81\xfa\x1f\x8e\xf0\x81\x02\x0f\xc3\xae\x38\x6d\x6c\xab\x53\xa5\xf5\xce\x07\x1a\x50\x46\x18\x0b\xc8\x5a\xe3\x9a\x6b\x84\x2c\x77\xb6\x8a\x1e\xe5\xc9\xe1\xcb\xb1\x76\x75\x6f\xcc\xb5\xad\x01\xc1\x07\x14\x1b\x73\x3d\x11\x10\x12\x22\x8f\x63\x25\xb8\x41\xb8\xb5\x77\xfc\x51\x6e\xa4\x6d\x8e\x2e\xb7\x9f\x87\x7d\xc7\xb5\x58\x71\xed\xfd\x99\xcd\xd8\x77\xea\x5f\x2d\x75\xa1\x92\x24\x32\xc1\x5b\x00\x00\x00\xff\xff\x60\xc5\x22\xd5\x2d\x04\x00\x00")

func _000002_add_milestoneUpSqlBytes() ([]byte, error) {
	return bindataRead(
		__000002_add_milestoneUpSql,
		"000002_add_milestone.up.sql",
	)
}

func _000002_add_milestoneUpSql() (*asset, error) {
	bytes, err := _000002_add_milestoneUpSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000002_add_milestone.up.sql", size: 0, mode: os.FileMode(0644), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x2, 0xa5, 0x3e, 0x3d, 0x2d, 0x5f, 0xfe, 0x41, 0x3c, 0xc3, 0xa7, 0x9f, 0xc3, 0x7d, 0xf1, 0x46, 0x8, 0x9d, 0x12, 0x3f, 0x9f, 0xd2, 0xab, 0xe5, 0xb1, 0x23, 0x1a, 0xe7, 0x37, 0x9f, 0xdb, 0x66}}
	return a, nil
}

var __000003_drop_spinmint_tableDownSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x84\xd0\xc1\x4b\xc3\x30\x14\xc7\xf1\x7b\xfe\x8a\xdf\xb1\x05\x0f\xb6\x38\x18\x8c\x1d\xd2\xee\x6d\x3e\x6c\x53\x49\x33\x70\xb7\xa4\x5b\xd4\x1e\x9a\x8d\x9a\xe9\xbf\x2f\x55\x50\xc1\xc1\xee\x9f\xef\x83\xf7\x2b\x68\xc3\x6a\x21\x44\xa9\x49\x1a\x82\x91\x45\x45\xe0\x35\x54\x63\x40\x4f\xdc\x9a\x16\xb6\x3d\xf5\x61\xe8\x43\xb4\x02\x48\x04\x00\x58\x0e\x6f\xd1\x85\xbd\xe7\x83\xc5\xbb\x1b\xf7\xaf\x6e\x4c\xb2\x7c\x9e\x7e\x75\x6a\x5b\x55\x37\xdf\x4e\xfb\xd3\xb1\xf9\x08\x7e\xfc\x65\xf9\x6c\x96\x62\x45\x6b\xb9\xad\xfe\x51\xe5\x06\x7f\x5d\xaa\xf3\xd0\x4d\x17\xfb\x10\x93\x2c\xbb\x48\xca\xd1\xbb\xe8\x0f\x32\x5a\x74\xfd\xcb\x04\xf3\xdb\x4b\xf0\x51\x73\x2d\xf5\x0e\x0f\xb4\x4b\xfe\x3e\x95\x0a\x20\x05\xa9\x0d\x2b\x5a\x72\x08\xc7\x55\xf1\x53\x97\xf7\x52\xb7\x64\x96\xe7\xf8\x3c\x1f\xba\xbb\x69\xbc\xa6\xae\xd9\x2c\x3e\x03\x00\x00\xff\xff\xf7\x0e\xa6\xce\x4c\x01\x00\x00")

func _000003_drop_spinmint_tableDownSqlBytes() ([]byte, error) {
	return bindataRead(
		__000003_drop_spinmint_tableDownSql,
		"000003_drop_spinmint_table.down.sql",
	)
}

func _000003_drop_spinmint_tableDownSql() (*asset, error) {
	bytes, err := _000003_drop_spinmint_tableDownSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000003_drop_spinmint_table.down.sql", size: 0, mode: os.FileMode(0644), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x52, 0xed, 0xc6, 0x89, 0x7, 0x19, 0x3b, 0x68, 0x70, 0x22, 0xcc, 0x3b, 0xd2, 0x93, 0x97, 0xab, 0x5, 0x4f, 0x22, 0xd5, 0xf7, 0x84, 0xe5, 0xc, 0x43, 0xfe, 0x50, 0x4d, 0x18, 0x6a, 0x49, 0x35}}
	return a, nil
}

var __000003_drop_spinmint_tableUpSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x72\x72\x75\xf7\xf4\xb3\xe6\xe2\x72\x09\xf2\x0f\x50\x08\x71\x74\xf2\x71\x55\xf0\x74\x53\x70\x8d\xf0\x0c\x0e\x09\x56\x48\x08\x2e\xc8\xcc\xcb\xcd\xcc\x2b\x49\xb0\xe6\xe2\x72\xf6\xf7\xf5\xf5\x0c\xb1\x06\x04\x00\x00\xff\xff\xe6\x87\xad\xaf\x31\x00\x00\x00")

func _000003_drop_spinmint_tableUpSqlBytes() ([]byte, error) {
	return bindataRead(
		__000003_drop_spinmint_tableUpSql,
		"000003_drop_spinmint_table.up.sql",
	)
}

func _000003_drop_spinmint_tableUpSql() (*asset, error) {
	bytes, err := _000003_drop_spinmint_tableUpSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "000003_drop_spinmint_table.up.sql", size: 0, mode: os.FileMode(0644), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x2d, 0xda, 0x7b, 0x68, 0xe9, 0x59, 0x60, 0xd5, 0xf3, 0x53, 0x1c, 0xdd, 0x26, 0x89, 0xdc, 0x6e, 0x5, 0x53, 0xd3, 0x2d, 0x9d, 0x53, 0xf4, 0x7e, 0xdf, 0x2a, 0x1c, 0xb0, 0xae, 0xb4, 0xf7, 0xc0}}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetString returns the asset contents as a string (instead of a []byte).
func AssetString(name string) (string, error) {
	data, err := Asset(name)
	return string(data), err
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// MustAssetString is like AssetString but panics when Asset would return an
// error. It simplifies safe initialization of global variables.
func MustAssetString(name string) string {
	return string(MustAsset(name))
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetDigest returns the digest of the file with the given name. It returns an
// error if the asset could not be found or the digest could not be loaded.
func AssetDigest(name string) ([sha256.Size]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s can't read by error: %v", name, err)
		}
		return a.digest, nil
	}
	return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s not found", name)
}

// Digests returns a map of all known files and their checksums.
func Digests() (map[string][sha256.Size]byte, error) {
	mp := make(map[string][sha256.Size]byte, len(_bindata))
	for name := range _bindata {
		a, err := _bindata[name]()
		if err != nil {
			return nil, err
		}
		mp[name] = a.digest
	}
	return mp, nil
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"000001_base.down.sql":                _000001_baseDownSql,
	"000001_base.up.sql":                  _000001_baseUpSql,
	"000002_add_milestone.down.sql":       _000002_add_milestoneDownSql,
	"000002_add_milestone.up.sql":         _000002_add_milestoneUpSql,
	"000003_drop_spinmint_table.down.sql": _000003_drop_spinmint_tableDownSql,
	"000003_drop_spinmint_table.up.sql":   _000003_drop_spinmint_tableUpSql,
}

// AssetDebug is true if the assets were built with the debug flag enabled.
const AssetDebug = false

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"},
// AssetDir("data/img") would return []string{"a.png", "b.png"},
// AssetDir("foo.txt") and AssetDir("notexist") would return an error, and
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		canonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(canonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"000001_base.down.sql":                {_000001_baseDownSql, map[string]*bintree{}},
	"000001_base.up.sql":                  {_000001_baseUpSql, map[string]*bintree{}},
	"000002_add_milestone.down.sql":       {_000002_add_milestoneDownSql, map[string]*bintree{}},
	"000002_add_milestone.up.sql":         {_000002_add_milestoneUpSql, map[string]*bintree{}},
	"000003_drop_spinmint_table.down.sql": {_000003_drop_spinmint_tableDownSql, map[string]*bintree{}},
	"000003_drop_spinmint_table.up.sql":   {_000003_drop_spinmint_tableUpSql, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory.
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	return os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
}

// RestoreAssets restores an asset under the given directory recursively.
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(canonicalName, "/")...)...)
}
