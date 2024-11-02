package models

import (
	"time"
)

type File struct {
    ID        uint      `json:"id" gorm:"primary_key"`
    Filename  string    `json:"filename"`
    Size      int64     `json:"size"`
    UploadedAt time.Time `json:"uploaded_at"`
    Hash  string    `json:"-"`
    FileID    string    `json:"-"`
}

func (File) TableName() string {
    return "files"
}
