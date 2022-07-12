package icopy

import (
	"bytes"
	"context"
	"encoding/binary"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

// mov spec: https://developer.apple.com/standards/qtff-2001.pdf
// Page 31-33 contain information used in this file

const appleEpochAdjustment = 2082844800

const (
	movieResourceAtomType   = "moov"
	movieHeaderAtomType     = "mvhd"
	referenceMovieAtomType  = "rmra"
	compressedMovieAtomType = "cmov"
)

func ReadVideoCreationTimeMetadata(ctx context.Context, src_dirname string, recursive bool) ([]FileObject, []ErroredFileObject) {

	de, err := os.ReadDir(src_dirname)
	if err != nil {
		log.Println("Failed to read directory", err)
		panic(err)
	}

	imageFiles := []FileObject{}
	erroredFiles := []ErroredFileObject{}

	for _, file := range de {
		if file.IsDir() {
			if recursive {
				log.Println("Recursively reading directory:", file.Name())
				imageFiles1, erroredFiles1 := ReadVideoCreationTimeMetadata(ctx, path.Join(src_dirname, file.Name()), recursive)
				imageFiles = append(imageFiles, imageFiles1...)
				erroredFiles = append(erroredFiles, erroredFiles1...)
			}
		} else {
			fpath := path.Join(src_dirname, file.Name())
			if strings.HasSuffix(strings.ToLower(file.Name()), ".mp4") || strings.HasSuffix(strings.ToLower(file.Name()), ".mov") {
				log.Printf("Reading file: %s", file.Name())

				videoBuffer, err := os.Open(fpath) // Open the source file for reading
				if err != nil {
					panic(err)
				}
				defer videoBuffer.Close()

				buf := make([]byte, 8)
				failed := false

				// Traverse videoBuffer to find movieResourceAtom
				for {
					// bytes 1-4 is atom size, 5-8 is type
					// Read atom
					if _, err := videoBuffer.Read(buf); err != nil {
						erroredFiles = append(erroredFiles, ErroredFileObject{
							DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
							ErrorMessage: err.Error(),
						})
						failed = true

						if err.Error() == "EOF" {
							fi, err := os.Stat(fpath)
							if err == nil {
								tm := fi.ModTime()
								imageFiles = append(imageFiles, FileObject{Name: file.Name(), Path: src_dirname, DateTime: tm})
							}
						}
						break
					}

					if bytes.Equal(buf[4:8], []byte(movieResourceAtomType)) {
						break // found it!
					}

					atomSize := binary.BigEndian.Uint32(buf) // check size of atom
					videoBuffer.Seek(int64(atomSize)-8, 1)   // jump over data and set seeker at beginning of next atom
				}

				if failed {
					continue
				}

				// read next atom
				if _, err := videoBuffer.Read(buf); err != nil {
					erroredFiles = append(erroredFiles, ErroredFileObject{
						DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
						ErrorMessage: err.Error(),
					})
					log.Println("Error reading movie atom: ", err)
					continue
				}

				atomType := string(buf[4:8]) // skip size and read type
				switch atomType {
				case movieHeaderAtomType:
					// read next atom
					if _, err := videoBuffer.Read(buf); err != nil {
						erroredFiles = append(erroredFiles, ErroredFileObject{
							DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
							ErrorMessage: err.Error(),
						})
						log.Println("Error reading movie header type: ", err)
						continue
					}

					// byte 1 is version, byte 2-4 is flags, 5-8 Creation time
					appleEpoch := int64(binary.BigEndian.Uint32(buf[4:])) // Read creation time

					tm := time.Unix(appleEpoch-appleEpochAdjustment, 0).Local()
					imageFiles = append(imageFiles, FileObject{Name: file.Name(), Path: src_dirname, DateTime: tm})
				default:
					erroredFiles = append(erroredFiles, ErroredFileObject{
						DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
						ErrorMessage: atomType + " is not a valid atom type",
					})
					log.Println("Error reading movie header all: ", err)
				}
			} else if strings.HasSuffix(strings.ToLower(file.Name()), ".wmv") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".avi") {
				log.Printf("Its WMV : %s", file.Name())
				fi, err := os.Stat(fpath)
				if err == nil {
					imageFiles = append(imageFiles, FileObject{Name: file.Name(), Path: src_dirname, DateTime: fi.ModTime()})
				}
			}
		}
	}
	return imageFiles, erroredFiles
}

// files, err := ioutil.ReadDir(src_dirname)
// if err != nil {
// 	panic(err)
// }

// imageFiles := []FileObject{}
// erroredFiles := []ErroredFileObject{}

// for _, file := range files {
// 	fpath := path.Join(src_dirname, file.Name())
// 	ext := strings.ToLower(fpath[len(fpath)-4:])

// 	if ext == ".mp4" || ext == ".mov" {
// 		videoBuffer, err := os.Open(fpath)
// 		if err != nil {
// 			panic(err)
// 		}
// 		defer videoBuffer.Close()

// 		buf := make([]byte, 8)
// 		failed := false

// 		// Traverse videoBuffer to find movieResourceAtom
// 		for {
// 			// bytes 1-4 is atom size, 5-8 is type
// 			// Read atom
// 			if _, err := videoBuffer.Read(buf); err != nil {
// 				erroredFiles = append(erroredFiles, ErroredFileObject{
// 					DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 					ErrorMessage: err.Error(),
// 				})
// 				failed = true
// 				break
// 			}

// 			if bytes.Equal(buf[4:8], []byte(movieResourceAtomType)) {
// 				break // found it!
// 			}

// 			atomSize := binary.BigEndian.Uint32(buf) // check size of atom
// 			videoBuffer.Seek(int64(atomSize)-8, 1)   // jump over data and set seeker at beginning of next atom
// 		}

// 		if failed {
// 			continue
// 		}

// 		// read next atom
// 		if _, err := videoBuffer.Read(buf); err != nil {
// 			erroredFiles = append(erroredFiles, ErroredFileObject{
// 				DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 				ErrorMessage: err.Error(),
// 			})
// 			continue
// 		}

// 		atomType := string(buf[4:8]) // skip size and read type
// 		switch atomType {
// 		case movieHeaderAtomType:
// 			// read next atom
// 			if _, err := videoBuffer.Read(buf); err != nil {
// 				erroredFiles = append(erroredFiles, ErroredFileObject{
// 					DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 					ErrorMessage: err.Error(),
// 				})
// 				continue
// 			}

// 			// byte 1 is version, byte 2-4 is flags, 5-8 Creation time
// 			appleEpoch := int64(binary.BigEndian.Uint32(buf[4:])) // Read creation time

// 			tm := time.Unix(appleEpoch-appleEpochAdjustment, 0).Local()
// 			imageFiles = append(imageFiles, FileObject{Name: file.Name(), Path: src_dirname, DateTime: tm})
// 		default:
// 			erroredFiles = append(erroredFiles, ErroredFileObject{
// 				DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 				ErrorMessage: atomType + " is not a valid atom type",
// 			})
// 		}
// 	}
// }
// return imageFiles, erroredFiles
