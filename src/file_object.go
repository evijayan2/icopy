package icopy

import "time"

type FileObject struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	DateTime time.Time `json:"date_time"`
	Md5Sum   string    `json:"md5sum"`
}

type ScanOptions struct {
	Recursive    bool
	NumWorkers   int
	UseFastHash  bool
	ProgressChan chan string
}

type ErroredFileObject struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	DateTime     time.Time `json:"date_time"`
	ErrorMessage string    `json:"error_message"`
}
