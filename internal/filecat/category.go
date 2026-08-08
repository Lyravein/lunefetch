// Package filecat maps file extensions to download categories.
// Category names are used as subfolder names under the base DownloadDir.
package filecat

import "strings"

// Category represents a file type group.
type Category string

const (
	Video    Category = "Videos"
	Audio    Category = "Music"
	Image    Category = "Images"
	Document Category = "Documents"
	Archive  Category = "Archives"
	Program  Category = "Programs"
	Other    Category = "Other"
)

// All returns all known categories in display order.
func All() []Category {
	return []Category{Video, Audio, Image, Document, Archive, Program, Other}
}

var extMap = map[string]Category{
	// Video
	".mp4": Video, ".mkv": Video, ".avi": Video, ".mov": Video,
	".wmv": Video, ".flv": Video, ".webm": Video, ".m4v": Video,
	".mpg": Video, ".mpeg": Video, ".3gp": Video, ".ts": Video,

	// Audio
	".mp3": Audio, ".flac": Audio, ".aac": Audio, ".ogg": Audio,
	".wav": Audio, ".wma": Audio, ".m4a": Audio, ".opus": Audio,
	".aiff": Audio, ".alac": Audio,

	// Image
	".jpg": Image, ".jpeg": Image, ".png": Image, ".gif": Image,
	".bmp": Image, ".webp": Image, ".tiff": Image, ".tif": Image,
	".svg": Image, ".ico": Image, ".heic": Image, ".raw": Image,

	// Document
	".pdf": Document, ".doc": Document, ".docx": Document,
	".xls": Document, ".xlsx": Document, ".ppt": Document, ".pptx": Document,
	".odt": Document, ".ods": Document, ".odp": Document,
	".txt": Document, ".md": Document, ".epub": Document, ".mobi": Document,
	".csv": Document,

	// Archive
	".zip": Archive, ".rar": Archive, ".7z": Archive, ".tar": Archive,
	".gz": Archive, ".bz2": Archive, ".xz": Archive, ".tgz": Archive,
	".tbz2": Archive, ".iso": Archive, ".dmg": Archive,

	// Program / installer
	".exe": Program, ".msi": Program, ".deb": Program, ".rpm": Program,
	".appimage": Program, ".apk": Program, ".pkg": Program, ".run": Program,
	".sh": Program, ".jar": Program,
}

// FromFilename returns the Category for a given filename based on its extension.
// Falls back to Other if the extension is unknown.
func FromFilename(filename string) Category {
	ext := strings.ToLower(extOf(filename))
	if cat, ok := extMap[ext]; ok {
		return cat
	}
	return Other
}

// FromURL is a convenience wrapper that extracts the last path segment of a URL
// and calls FromFilename on it.
func FromURL(rawURL string) Category {
	// strip query / fragment
	if i := strings.IndexByte(rawURL, '?'); i != -1 {
		rawURL = rawURL[:i]
	}
	if i := strings.IndexByte(rawURL, '#'); i != -1 {
		rawURL = rawURL[:i]
	}
	// last path segment
	if i := strings.LastIndexByte(rawURL, '/'); i != -1 {
		rawURL = rawURL[i+1:]
	}
	return FromFilename(rawURL)
}

func extOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
	}
	return ""
}
