package icopy

import (
	"bufio"
	"log"
	"os"
	"path"
)

func RemoveSourceFile(src_dirname string) []FileObject {

	fd, err := os.Open("./.file_status.txt")
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	filesRemoved := []FileObject{}

	for scanner.Scan() {
		line := scanner.Text()

		if fi, _ := os.Stat(line); fi != nil {

			err = os.Remove(line)
			if err != nil {
				log.Println("Not able to remove file:", line)
			} else {
				path, file := path.Split(line)
				filesRemoved = append(filesRemoved, FileObject{Name: file, Path: path, DateTime: fi.ModTime()})
			}
		}
	}

	if filesRemoved != nil {
		return filesRemoved
	}
	return nil
}
