// Package filecat maps file extensions to download categories.
// Category names identify file-type groups. They do not rename existing files.
package filecat

import "strings"

// Category represents a file type group.
type Category string

const (
	Media      Category = "Media"
	Document   Category = "Documents"
	Compressed Category = "Compressed"
	Program    Category = "Programs"
	Other      Category = "Other"
)

// All returns all known categories in display order.
func All() []Category {
	return []Category{Compressed, Document, Media, Program, Other}
}

var extMap = map[string]Category{
	// Media: video
	".mp4": Media, ".mkv": Media, ".avi": Media, ".mov": Media,
	".wmv": Media, ".flv": Media, ".webm": Media, ".m4v": Media,
	".mpg": Media, ".mpeg": Media, ".3gp": Media, ".ts": Media,

	// Audio
	".mp3": Media, ".flac": Media, ".aac": Media, ".ogg": Media,
	".wav": Media, ".wma": Media, ".m4a": Media, ".opus": Media,
	".aiff": Media, ".alac": Media,

	// Image
	".jpg": Media, ".jpeg": Media, ".png": Media, ".gif": Media,
	".bmp": Media, ".webp": Media, ".tiff": Media, ".tif": Media,
	".svg": Media, ".ico": Media, ".heic": Media, ".raw": Media,

	// Document
	".pdf": Document, ".doc": Document, ".docx": Document,
	".xls": Document, ".xlsx": Document, ".ppt": Document, ".pptx": Document,
	".odt": Document, ".ods": Document, ".odp": Document,
	".txt": Document, ".md": Document, ".epub": Document, ".mobi": Document,
	".csv": Document,

	// Compressed
	".zip": Compressed, ".rar": Compressed, ".7z": Compressed, ".tar": Compressed,
	".gz": Compressed, ".bz2": Compressed, ".xz": Compressed, ".tgz": Compressed,
	".tbz2": Compressed, ".iso": Compressed, ".dmg": Compressed,

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
