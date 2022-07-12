package icopy

import "sort"

func SortFilesByDate(files []FileObject) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].DateTime.Before(files[j].DateTime)
	},
	)
}
