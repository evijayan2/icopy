package icopy

import "time"

type FileObject struct {
	DateTime     time.Time
	Name         string
	Path         string
	Md5Sum       string
	ErrorMessage string
}

type ErroredFileObject FileObject
