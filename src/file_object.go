package icopy

import "time"

type FileObject struct {
	DateTime     time.Time
	Name         string
	Path         string
	ErrorMessage string
}

type ErroredFileObject FileObject
