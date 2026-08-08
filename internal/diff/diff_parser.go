package diff

import (
	"strconv"
	"strings"
)

type FileType int

const (
	Normal FileType = iota
	Binary
	Delete
	Rename
	New
)

type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

type File struct {
	Name     string
	Type     FileType
	RenameTo string
	Hunks    []Hunk
}

type Hunk struct {
	Context  string
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []Line
}

type Line struct {
	Type    LineType
	Content string
}

// Parse turns raw `git diff` output into structured File values.
func Parse(raw string) []File {
	sections := splitDiffSections(raw)
	files := make([]File, 0, len(sections))
	for _, section := range sections {
		f := parseFileSection(section)
		if f.Name == "" {
			continue
		}
		files = append(files, f)
	}
	return files
}

// splitDiffSections returns one string per file, each keeping its "diff --git"
// line.
func splitDiffSections(raw string) []string {
	const marker = "diff --git "
	var sections []string
	rest := raw
	for {
		idx := strings.Index(rest, marker)
		if idx == -1 {
			break
		}
		rest = rest[idx:]
		next := strings.Index(rest[1:], marker)
		if next == -1 {
			sections = append(sections, rest)
			break
		}
		sections = append(sections, rest[:next+1])
		rest = rest[next+1:]
	}
	return sections
}

func parseFileSection(section string) File {
	lines := strings.Split(section, "\n")
	if len(lines) == 0 || lines[0] == "" {
		return File{}
	}
	f := File{Type: Normal}

	aName, bName := parseGitHeader(lines[0])
	f.Name = aName

	if f.detectFileType(lines[1:], bName) {
		return f // binary file — no hunks to parse
	}

	f.Hunks = parseHunks(lines)
	return f
}

// detectFileType reads the headers before the first @@, returning true when there
// is nothing further to parse (binary files have no hunks).
func (f *File) detectFileType(headerLines []string, bName string) bool {
	hasSimilarityIndex := false
	for _, line := range headerLines {
		if strings.HasPrefix(line, "@@") {
			break
		}
		switch {
		case strings.HasPrefix(line, "Binary files"):
			f.Type = Binary
			return true
		case strings.HasPrefix(line, "deleted file mode"):
			f.Type = Delete
		case strings.HasPrefix(line, "new file mode"):
			f.Type = New
		case strings.HasPrefix(line, "rename from"):
			f.Type = Rename
		case strings.HasPrefix(line, "rename to "):
			f.RenameTo = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "similarity index"):
			hasSimilarityIndex = true
		}
	}

	if hasSimilarityIndex && f.Name != bName {
		f.Type = Rename
		if f.RenameTo == "" {
			f.RenameTo = bName
		}
	}
	return false
}

// parseGitHeader pulls both filenames out of a "diff --git a/X b/Y" line.
func parseGitHeader(header string) (aName, bName string) {
	rest := strings.TrimPrefix(header, "diff --git ")

	// Paths may contain spaces, so " b/" is the only reliable split point. The
	// search starts past the leading "a/" so a path of its own cannot match.
	sep := " b/"
	idx := strings.Index(rest[2:], sep)
	if idx == -1 {
		// Unreachable for valid diff output.
		name := strings.TrimPrefix(rest, "a/")
		return name, name
	}
	aName = rest[2 : idx+2] // skip leading "a/"
	bName = rest[idx+2+len(sep):]
	return aName, bName
}

func parseHunks(lines []string) []Hunk {
	var hunks []Hunk
	var current *Hunk

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			h := parseHunkHeader(line)
			hunks = append(hunks, h)
			current = &hunks[len(hunks)-1]
			continue
		}
		if current == nil {
			continue
		}
		dl, ok := classifyLine(line)
		if !ok {
			continue
		}
		current.Lines = append(current.Lines, dl)
	}
	return hunks
}

// Parses "@@ -old,count +new,count @@ context".
func parseHunkHeader(line string) Hunk {
	var h Hunk
	rest := line[2:] // skip leading "@@"
	end := strings.Index(rest, "@@")
	if end == -1 {
		return h
	}
	rangePart := strings.TrimSpace(rest[:end])
	h.Context = strings.TrimSpace(rest[end+2:])

	// rangePart looks like "-10,6 +10,7".
	parts := strings.Fields(rangePart)
	if len(parts) >= 1 {
		h.OldStart, h.OldCount = parseRange(parts[0])
	}
	if len(parts) >= 2 {
		h.NewStart, h.NewCount = parseRange(parts[1])
	}
	return h
}

// Parses "-10,6" or "+10,7" into start and count.
func parseRange(s string) (int, int) {
	s = strings.TrimLeft(s, "-+")
	if strings.Contains(s, ",") {
		parts := strings.SplitN(s, ",", 2)
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0
		}
		count, err := strconv.Atoi(parts[1])
		if err != nil {
			return start, 0
		}
		return start, count
	}
	start, err := strconv.Atoi(s)
	if err != nil {
		return 0, 1
	}
	return start, 1
}

// classifyLine returns false for lines that are not diff content, such as
// "\ No newline at end of file".
func classifyLine(line string) (Line, bool) {
	if len(line) == 0 {
		return Line{}, false
	}
	switch line[0] {
	case '+':
		return Line{Type: LineAdded, Content: line[1:]}, true
	case '-':
		return Line{Type: LineRemoved, Content: line[1:]}, true
	case ' ':
		return Line{Type: LineContext, Content: line[1:]}, true
	default:
		return Line{}, false
	}
}
