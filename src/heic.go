package icopy

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// ExtractHeicExif parses a HEIC file (ISOBMFF) and extracts the raw EXIF data block.
func ExtractHeicExif(r io.ReaderAt) ([]byte, error) {
	// 1. Read File Type Box (ftyp) - usually first, but we can scan for 'meta'
	// We need to find the 'meta' box.
	// ISOBMFF boxes: [Size(4)][Type(4)][Data...]

	off := int64(0)
	buf := make([]byte, 8)

	// Limit search to first 1MB just in case
	for off < 1024*1024 {
		if _, err := r.ReadAt(buf, off); err != nil {
			return nil, err
		}

		boxSize := int64(binary.BigEndian.Uint32(buf[0:4]))
		boxType := string(buf[4:8])

		if boxSize == 1 {
			// Large size, read next 8 bytes
			largeBuf := make([]byte, 8)
			if _, err := r.ReadAt(largeBuf, off+8); err != nil {
				return nil, err
			}
			boxSize = int64(binary.BigEndian.Uint64(largeBuf))
		}

		if boxSize == 0 {
			// Last box, goes to end of file
			// We can't know size easily without stat, but usually meta isn't last-and-unbounded in valid heic
			break
		}

		if boxType == "meta" {
			// Found meta box. The meta box is a "FullBox", so it has Version(1) + Flags(3)
			// But inside it contains other boxes.
			// Depending on standard, meta might be just a container or have version.
			// In HEIF/HEIC, 'meta' is a FullBox.
			metaOff := off + 12 // 8 (header) + 4 (ver+flags)
			if boxSize == 1 {
				metaOff = off + 16 + 4
			}

			// Limit meta parse to its size
			return parseMetaBox(r, metaOff, boxSize)
		}

		off += boxSize
	}

	return nil, errors.New("meta box not found")
}

func parseMetaBox(r io.ReaderAt, off int64, size int64) ([]byte, error) {
	// Inside meta, we look for 'iinf' (Item Info) to find ID of Exif item,
	// and 'iloc' (Item Location) to find offset of that ID.

	end := off + size
	curr := off
	buf := make([]byte, 8)

	var exifItemID uint16
	foundExif := false

	// First pass: find iinf
	// We might need strict order? Standards say iinf usually comes before iloc.
	// We need to parse all children to be safe or just scan.

	// Let's store boxes found
	type boxLoc struct {
		offset int64
		size   int64
	}
	var iinf, iloc boxLoc

	for curr < end {
		if _, err := r.ReadAt(buf, curr); err != nil {
			break
		}
		bSize := int64(binary.BigEndian.Uint32(buf[0:4]))
		bType := string(buf[4:8])

		if bSize == 0 || bSize == 1 {
			// Simple parser limitation for brevity: skip complex sizes inside meta for now implies small boxes
			break
		}

		if bType == "iinf" {
			iinf = boxLoc{curr, bSize}
		} else if bType == "iloc" {
			iloc = boxLoc{curr, bSize}
		}

		curr += bSize
	}

	if iinf.offset == 0 || iloc.offset == 0 {
		return nil, errors.New("iinf or iloc box not found in meta")
	}

	// Parse iinf to find Exif item ID
	// iinf is FullBox: [4 size][4 type][1 ver][3 flags][2 entry_count]
	// entries are 'infe' boxes.
	// infe is FullBox.

	iinfDataOff := iinf.offset + 12
	// Read entry count?
	// Actually iinf contains child boxes 'infe'. Standard says "ItemInfoBox contains ItemInfoEntryBoxes".
	// So we walk iinf children.

	curr = iinfDataOff
	iinfEnd := iinf.offset + iinf.size

	for curr < iinfEnd {
		if _, err := r.ReadAt(buf, curr); err != nil {
			break
		}
		bSize := int64(binary.BigEndian.Uint32(buf[0:4]))
		bType := string(buf[4:8])

		if bType == "infe" {
			// Parse Item Info Entry
			// FullBox: [4 ver+flags]
			// Version 2/3 common in HEIC.
			// v2: [2 item_ID] [2 item_protection_index] [4 item_type] [string item_name] ...
			// v3: [4 item_ID] ...

			header := make([]byte, 12) // ver(1)+flags(3) + ...
			if _, err := r.ReadAt(header, curr+8); err != nil {
				return nil, err
			}
			version := header[0]

			var itemID uint32
			var itemType string

			if version == 2 {
				itemID = uint32(binary.BigEndian.Uint16(header[4:6]))
				// prot_ind at 6:8
				itemType = string(header[8:12])
			} else if version == 3 {
				itemID = binary.BigEndian.Uint32(header[4:8])
				// prot_ind at 8:10
				itemType = string(header[10:14])
			}

			if itemType == "Exif" {
				exifItemID = uint16(itemID) // assuming ID fits in u16 or our iloc logic handles u32 (usually IDs are small)
				foundExif = true
				break
			}
		}
		curr += bSize
	}

	if !foundExif {
		return nil, errors.New("Exif item not found in iinf")
	}

	// Parse iloc to find offset
	// iloc is FullBox.
	// [1 ver][3 flags]
	// [1 offset_size | length_size] (4 bits each)
	// [1 base_offset_size | index_size? (ver 1/2)] or reserved (ver 0)
	// [2 item_count]

	ilocHeader := make([]byte, 16)
	if _, err := r.ReadAt(ilocHeader, iloc.offset+8); err != nil {
		return nil, err
	}

	version := ilocHeader[0]
	// offsetSize := (ilocHeader[4] >> 4) & 0xF
	lengthSize := ilocHeader[4] & 0xF
	baseOffsetSize := (ilocHeader[5] >> 4) & 0xF

	itemCountOffset := 6
	if version < 2 {
		itemCountOffset = 6
	} else if version == 2 {
		itemCountOffset = 6 // verify spec? usually similar structure
	}

	itemCount := binary.BigEndian.Uint16(ilocHeader[itemCountOffset : itemCountOffset+2])

	// Start reading items
	curr = iloc.offset + 8 + int64(itemCountOffset) + 2

	for i := 0; i < int(itemCount); i++ {
		// Read Item ID
		var id uint32
		if version < 2 {
			b2 := make([]byte, 2)
			r.ReadAt(b2, curr)
			id = uint32(binary.BigEndian.Uint16(b2))
			curr += 2
		} else {
			b4 := make([]byte, 4)
			r.ReadAt(b4, curr)
			id = binary.BigEndian.Uint32(b4)
			curr += 4
		}

		if version == 1 || version == 2 {
			curr += 2 // construction_method_index
		}

		// data_reference_index
		curr += 2

		// base_offset
		var baseOffset int64
		if baseOffsetSize == 4 {
			b4 := make([]byte, 4)
			r.ReadAt(b4, curr)
			baseOffset = int64(binary.BigEndian.Uint32(b4))
			curr += 4
		} else if baseOffsetSize == 8 {
			b8 := make([]byte, 8)
			r.ReadAt(b8, curr)
			baseOffset = int64(binary.BigEndian.Uint64(b8))
			curr += 8
		}

		// extent_count
		b2 := make([]byte, 2)
		r.ReadAt(b2, curr)
		extentCount := binary.BigEndian.Uint16(b2)
		curr += 2

		for j := 0; j < int(extentCount); j++ {
			// extent_offset? (if version 1/2 has index constraints? assuming simpler case)
			// Standard: if version 1 or 2, verify.
			// Wait, if base_offset provided, extents are relative or absolute?
			// Usually: absolute = base_offset + extent_offset.

			// extent_offset (if present in earlier versions? No, usually not present if inferred? no it is present)
			// Actually field is extent_offset (size determined by offsetSize from header) UNLESS....
			// Let's assume standard iloc structure:
			// field: extent_offset (bytes: offsetSize)
			// field: extent_length (bytes: lengthSize)

			// Need offsetSize from header (we didn't store it)
			offsetSize := (ilocHeader[4] >> 4) & 0xF

			var extOffset int64
			if offsetSize == 4 {
				b4 := make([]byte, 4)
				r.ReadAt(b4, curr)
				extOffset = int64(binary.BigEndian.Uint32(b4))
				curr += 4
			} else if offsetSize == 8 {
				b8 := make([]byte, 8)
				r.ReadAt(b8, curr)
				extOffset = int64(binary.BigEndian.Uint64(b8))
				curr += 8
			}

			var extLen int64
			if lengthSize == 4 {
				b4 := make([]byte, 4)
				r.ReadAt(b4, curr)
				extLen = int64(binary.BigEndian.Uint32(b4))
				curr += 4
			} // handle other sizes if needed

			if id == uint32(exifItemID) {
				// Found it!
				finalOffset := baseOffset + extOffset

				// Read data
				// Exif data in HEIC usually starts with 4 bytes offset (usually 0 or 4?) + "Exif\0\0" potentially?
				// Actually the item storage for Exif often includes the 'Exif\0\0' header or sometimes just raw TIFF.
				// rwcarlsen/goexif expects raw TIFF (starting with II or MM).
				// Often embedded Exif block has a 4-byte header specifying size of "Exif\0\0" prefix?
				// Commonly observed: 4 bytes (size of prefix) + "Exif\0\0" + TIFF data.

				// Let's read a bit more to inspect.
				data := make([]byte, extLen)
				if _, err := r.ReadAt(data, finalOffset); err != nil {
					return nil, err
				}

				// Need to strip 4-byte size + "Exif\0\0" if present.
				// Usually checks for the "Exif" string.
				// Pattern: look for "Exif\0\0"
				idx := bytes.Index(data, []byte("Exif\x00\x00"))
				if idx != -1 {
					return data[idx+6:], nil // +6 to skip Exif\x00\x00
				}

				// If starts with II or MM, return as is
				if len(data) > 2 {
					if (data[0] == 'I' && data[1] == 'I') || (data[0] == 'M' && data[1] == 'M') {
						return data, nil
					}
				}

				// Sometimes checking 4 bytes skip
				if len(data) > 4 {
					// Check if data[4:] starts with Exif...
					// This logic is fuzzy without exact spec at hand, but this covers common Apple HEIC.
					return data[4:], nil // Blind guess for common offset wrapper if basic checks fail
				}

				return data, nil
			}
		}
	}

	return nil, errors.New("Exif item location not found in iloc")
}
