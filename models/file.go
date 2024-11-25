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
	UploadedBy int64        `json:"uploaded_by"`
	Hash       string       `json:"hash"`
	FileID     string       `json:"-"` // GridFS中的fileID
	ExpiredAt  sql.NullTime `json:"expired_at"`
	NoteID     string       `json:"note_id"` // 笔记id
}

func (File) TableName() string {
	return "files"
}

type DownFile struct {
	ID           uint      `json:"-" gorm:"primary_key"`
	FileID       uint      `json:"file_id"` // files表中的记录id
	DownloadedAt time.Time `json:"downloaded_at"`
	DownloadedBy int64     `json:"downloaded_by"`
}

func (DownFile) TableName() string {
	return "downloaded_files"
}

type FileWithDownloadInfo struct {
	File
	DownloadedAt time.Time `json:"downloaded_at"`
}
