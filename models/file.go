package models

import (
	"database/sql"
	"time"
)

type File struct {
	ID         uint         `json:"-" gorm:"primary_key"`
	Filename   string       `json:"filename"`
	Size       int64        `json:"size"`
	UploadedAt time.Time    `json:"uploaded_at"`
	Hash       string       `json:"hash"`
	FileID     string       `json:"-"` // GridFS中的fileID
	ExpiredAt  sql.NullTime `json:"expired_at"`
}

func (File) TableName() string {
	return "files"
}
