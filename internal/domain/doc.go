package domain

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// displayWidth returns the display width of a string (rune count).
// This is more accurate than len() for unicode characters like — and ↔.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// Document represents a plain text document.
// Path is relative to the docs root (e.g. "specs/auth.md", "notes.md").
type Document struct {
	Path  string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HeadingInfo represents a markdown heading found in a document.
type HeadingInfo struct {
	Heading    string
	Level      int
	LineNumber int
}

// SearchResult represents a single line match from a content search.
type SearchResult struct {
	Path          string
	LineNumber    int
	LineContent   string
	ContextBefore []string
	ContextAfter  []string
}

// lines splits Content into lines.
func (d *Document) lines() []string {
	if d.Content == "" {
		return []string{}
	}
	return strings.Split(d.Content, "\n")
}

// setLines joins lines back into Content.
func (d *Document) setLines(lines []string) {
	d.Content = strings.Join(lines, "\n")
}

// validateLineRange checks that a 1-based line range is valid.
func (d *Document) validateLineRange(startLine int, endLine int) error {
	count := d.GetLineCount()
	if startLine < 1 || startLine > count {
		return ErrLineOutOfRange
	}
	if endLine < startLine || endLine > count {
		return ErrLineOutOfRange
	}
	return nil
}

// resolveLineRange converts startLine/endLine where 0 means full document
// into actual 1-based line numbers. Returns the resolved start, end, and any error.
func (d *Document) resolveLineRange(startLine int, endLine int) (int, int, error) {
	count := d.GetLineCount()
	if startLine == 0 {
		startLine = 1
	}
	if endLine == 0 {
		endLine = count
	}
	if count == 0 {
		return 0, 0, nil
	}
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return 0, 0, err
	}
	return startLine, endLine, nil
}

// --- Document read methods ---

// GetLines returns the content of a specific line range (1-based, inclusive).
func (d *Document) GetLines(startLine int, endLine int) (string, error) {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return "", err
	}
	lines := d.lines()
	selected := lines[startLine-1 : endLine]
	return strings.Join(selected, "\n"), nil
}

// GetLineCount returns the total number of lines.
func (d *Document) GetLineCount() int {
	if d.Content == "" {
		return 0
	}
	return len(d.lines())
}

// FindLine returns all 1-based line numbers where text appears.
func (d *Document) FindLine(text string) ([]int, error) {
	lines := d.lines()
	var results []int
	for i, line := range lines {
		if strings.Contains(line, text) {
			results = append(results, i+1)
		}
	}
	return results, nil
}

// FindLineRegex returns all 1-based line numbers matching a regex pattern.
func (d *Document) FindLineRegex(pattern string) ([]int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, ErrInvalidRegex
	}
	lines := d.lines()
	var results []int
	for i, line := range lines {
		if re.MatchString(line) {
			results = append(results, i+1)
		}
	}
	return results, nil
}

// GetTableOfContents returns all markdown headings with line numbers and levels.
func (d *Document) GetTableOfContents() ([]HeadingInfo, error) {
	lines := d.lines()
	var toc []HeadingInfo
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for _, ch := range trimmed {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		if level > 0 && level <= 6 {
			heading := strings.TrimSpace(trimmed[level:])
			if heading != "" {
				toc = append(toc, HeadingInfo{
					Heading:    heading,
					Level:      level,
					LineNumber: i + 1,
				})
			}
		}
	}
	return toc, nil
}

// --- Document replace methods ---

// ReplaceNth replaces the Nth occurrence of oldText with newText.
// n defaults to 1 if <= 0. startLine/endLine of 0 means full document.
func (d *Document) ReplaceNth(oldText string, newText string, n int, startLine int, endLine int) error {
	if n <= 0 {
		n = 1
	}
	startLine, endLine, err := d.resolveLineRange(startLine, endLine)
	if err != nil {
		return err
	}
	if d.GetLineCount() == 0 {
		return ErrEditNoMatch
	}

	lines := d.lines()
	section := strings.Join(lines[startLine-1:endLine], "\n")

	count := 0
	idx := 0
	for {
		pos := strings.Index(section[idx:], oldText)
		if pos == -1 {
			break
		}
		count++
		if count == n {
			absPos := idx + pos
			section = section[:absPos] + newText + section[absPos+len(oldText):]
			lines = append(lines[:startLine-1], append(strings.Split(section, "\n"), lines[endLine:]...)...)
			d.setLines(lines)
			return nil
		}
		idx += pos + len(oldText)
	}

	if count == 0 {
		return ErrEditNoMatch
	}
	return ErrNthOutOfRange
}

// ReplaceAll replaces all occurrences of oldText with newText.
// startLine/endLine of 0 means full document.
func (d *Document) ReplaceAll(oldText string, newText string, startLine int, endLine int) error {
	startLine, endLine, err := d.resolveLineRange(startLine, endLine)
	if err != nil {
		return err
	}
	if d.GetLineCount() == 0 {
		return ErrEditNoMatch
	}

	lines := d.lines()
	section := strings.Join(lines[startLine-1:endLine], "\n")

	if !strings.Contains(section, oldText) {
		return ErrEditNoMatch
	}

	section = strings.ReplaceAll(section, oldText, newText)
	lines = append(lines[:startLine-1], append(strings.Split(section, "\n"), lines[endLine:]...)...)
	d.setLines(lines)
	return nil
}

// ReplaceRegex performs a regex find/replace on all matches.
// Supports capture groups ($1, $2) and (?i) for case-insensitive.
// startLine/endLine of 0 means full document.
func (d *Document) ReplaceRegex(pattern string, replacement string, startLine int, endLine int) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ErrInvalidRegex
	}

	startLine, endLine, err = d.resolveLineRange(startLine, endLine)
	if err != nil {
		return err
	}
	if d.GetLineCount() == 0 {
		return ErrEditNoMatch
	}

	lines := d.lines()
	section := strings.Join(lines[startLine-1:endLine], "\n")

	if !re.MatchString(section) {
		return ErrEditNoMatch
	}

	section = re.ReplaceAllString(section, replacement)
	lines = append(lines[:startLine-1], append(strings.Split(section, "\n"), lines[endLine:]...)...)
	d.setLines(lines)
	return nil
}

// --- Document line methods ---

// InsertLines inserts content at a specific line number (1-based).
// Content is inserted before the specified line. Use lineNum > line count to append.
func (d *Document) InsertLines(lineNum int, content string) error {
	lines := d.lines()
	count := len(lines)

	if d.Content == "" && lineNum == 1 {
		d.Content = content
		return nil
	}

	if lineNum < 1 || lineNum > count+1 {
		return ErrLineOutOfRange
	}

	newLines := strings.Split(content, "\n")
	result := make([]string, 0, count+len(newLines))
	result = append(result, lines[:lineNum-1]...)
	result = append(result, newLines...)
	result = append(result, lines[lineNum-1:]...)
	d.setLines(result)
	return nil
}

// MoveLines moves a line range to a new position (1-based).
// targetLine is the line number where the moved block will be inserted.
func (d *Document) MoveLines(startLine int, endLine int, targetLine int) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	count := len(lines)
	if targetLine < 1 || targetLine > count+1 {
		return ErrLineOutOfRange
	}

	// Extract the block
	block := make([]string, endLine-startLine+1)
	copy(block, lines[startLine-1:endLine])

	// Remove the block
	remaining := make([]string, 0, count-len(block))
	remaining = append(remaining, lines[:startLine-1]...)
	remaining = append(remaining, lines[endLine:]...)

	// Adjust target line after removal
	adjustedTarget := targetLine
	if targetLine > endLine {
		adjustedTarget -= len(block)
	} else if targetLine > startLine {
		adjustedTarget = startLine
	}

	if adjustedTarget < 1 {
		adjustedTarget = 1
	}
	if adjustedTarget > len(remaining)+1 {
		adjustedTarget = len(remaining) + 1
	}

	// Insert at adjusted target
	result := make([]string, 0, count)
	result = append(result, remaining[:adjustedTarget-1]...)
	result = append(result, block...)
	result = append(result, remaining[adjustedTarget-1:]...)
	d.setLines(result)
	return nil
}

// DeleteLines removes a line range (1-based, inclusive).
func (d *Document) DeleteLines(startLine int, endLine int) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	result := make([]string, 0, len(lines)-(endLine-startLine+1))
	result = append(result, lines[:startLine-1]...)
	result = append(result, lines[endLine:]...)
	d.setLines(result)
	return nil
}

// CopyLines duplicates a line range to a new position (1-based).
func (d *Document) CopyLines(startLine int, endLine int, targetLine int) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	if targetLine < 1 || targetLine > len(lines)+1 {
		return ErrLineOutOfRange
	}

	block := make([]string, endLine-startLine+1)
	copy(block, lines[startLine-1:endLine])

	result := make([]string, 0, len(lines)+len(block))
	result = append(result, lines[:targetLine-1]...)
	result = append(result, block...)
	result = append(result, lines[targetLine-1:]...)
	d.setLines(result)
	return nil
}

// IndentLines indents or outdents a line range.
// Positive levels prepend the prefix; negative levels remove it.
// Lines without the prefix on outdent are left unchanged.
func (d *Document) IndentLines(startLine int, endLine int, levels int, prefix string) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	for i := startLine - 1; i < endLine; i++ {
		if levels > 0 {
			for j := 0; j < levels; j++ {
				lines[i] = prefix + lines[i]
			}
		} else {
			for j := 0; j < -levels; j++ {
				if strings.HasPrefix(lines[i], prefix) {
					lines[i] = lines[i][len(prefix):]
				}
			}
		}
	}
	d.setLines(lines)
	return nil
}

// SortLines alphabetically sorts lines in a range (1-based).
func (d *Document) SortLines(startLine int, endLine int, ascending bool) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	section := lines[startLine-1 : endLine]
	if ascending {
		sort.Strings(section)
	} else {
		sort.Sort(sort.Reverse(sort.StringSlice(section)))
	}
	d.setLines(lines)
	return nil
}

// WrapLines wraps each line in a range with a prefix and suffix.
func (d *Document) WrapLines(startLine int, endLine int, prefix string, suffix string) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	for i := startLine - 1; i < endLine; i++ {
		lines[i] = prefix + lines[i] + suffix
	}
	d.setLines(lines)
	return nil
}

// --- Document markdown methods ---

// CheckCheckbox toggles a markdown checkbox at a line (1-based).
func (d *Document) CheckCheckbox(lineNum int, checked bool) error {
	lines := d.lines()
	if lineNum < 1 || lineNum > len(lines) {
		return ErrLineOutOfRange
	}
	line := lines[lineNum-1]
	trimmed := strings.TrimSpace(line)

	if checked {
		if strings.Contains(trimmed, "- [ ]") {
			lines[lineNum-1] = strings.Replace(line, "- [ ]", "- [x]", 1)
			d.setLines(lines)
			return nil
		}
		if strings.Contains(trimmed, "- [x]") {
			return nil // already checked
		}
		return ErrNotACheckbox
	}

	// unchecking
	if strings.Contains(trimmed, "- [x]") {
		lines[lineNum-1] = strings.Replace(line, "- [x]", "- [ ]", 1)
		d.setLines(lines)
		return nil
	}
	if strings.Contains(trimmed, "- [ ]") {
		return nil // already unchecked
	}
	return ErrNotACheckbox
}

// Renumber renumbers or reletters matching prefixed lines in a range.
// start is "1", "a", "A", etc. prefix is matched at line start (e.g. "- ", "## ").
func (d *Document) Renumber(startLine int, endLine int, prefix string, start string) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()

	isAlpha := false
	startNum := 0
	startChar := byte('a')
	if len(start) == 1 && start[0] >= 'a' && start[0] <= 'z' {
		isAlpha = true
		startChar = start[0]
	} else if len(start) == 1 && start[0] >= 'A' && start[0] <= 'Z' {
		isAlpha = true
		startChar = start[0]
	} else {
		for _, ch := range start {
			if ch >= '0' && ch <= '9' {
				startNum = startNum*10 + int(ch-'0')
			}
		}
	}

	counter := 0
	for i := startLine - 1; i < endLine; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		// Find existing number/letter after prefix
		rest := trimmed[len(prefix):]
		// Find the end of the number/letter portion
		numEnd := 0
		for numEnd < len(rest) && ((rest[numEnd] >= '0' && rest[numEnd] <= '9') ||
			(rest[numEnd] >= 'a' && rest[numEnd] <= 'z') ||
			(rest[numEnd] >= 'A' && rest[numEnd] <= 'Z')) {
			numEnd++
			// Only consume one letter for alpha mode
			if isAlpha {
				break
			}
		}
		if numEnd == 0 {
			continue
		}

		var newLabel string
		if isAlpha {
			newLabel = string(rune(int(startChar) + counter))
		} else {
			newLabel = strings.Repeat("", 0)
			n := startNum + counter
			newLabel = intToString(n)
		}
		counter++

		indent := lines[i][:len(lines[i])-len(trimmed)]
		lines[i] = indent + prefix + newLabel + rest[numEnd:]
	}
	d.setLines(lines)
	return nil
}

// RenumberRegex renumbers lines matching a regex pattern. The first capture group
// is replaced with sequential values starting from `start`. Preserves zero-padding
// based on the width of the start value (e.g. start="0023" → "0023", "0024", "0025").
func (d *Document) RenumberRegex(startLine int, endLine int, pattern string, start string) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ErrInvalidRegex
	}
	if re.NumSubexp() < 1 {
		return ErrInvalidRegex
	}

	lines := d.lines()

	isAlpha := false
	startNum := 0
	startChar := byte('a')
	padWidth := len(start) // preserve zero-padding width

	if len(start) == 1 && ((start[0] >= 'a' && start[0] <= 'z') || (start[0] >= 'A' && start[0] <= 'Z')) {
		isAlpha = true
		startChar = start[0]
	} else {
		for _, ch := range start {
			if ch >= '0' && ch <= '9' {
				startNum = startNum*10 + int(ch-'0')
			}
		}
	}

	counter := 0
	for i := startLine - 1; i < endLine; i++ {
		loc := re.FindStringSubmatchIndex(lines[i])
		if loc == nil {
			continue
		}
		// loc[2] and loc[3] are the start/end of the first capture group
		groupStart, groupEnd := loc[2], loc[3]

		var newLabel string
		if isAlpha {
			newLabel = string(rune(int(startChar) + counter))
		} else {
			n := startNum + counter
			s := intToString(n)
			// Zero-pad to match start value width
			for len(s) < padWidth {
				s = "0" + s
			}
			newLabel = s
		}
		counter++

		lines[i] = lines[i][:groupStart] + newLabel + lines[i][groupEnd:]
	}
	d.setLines(lines)
	return nil
}

// intToString converts an int to its string representation.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// findHeading finds a heading line by its text (without # prefix).
// Returns the 1-based line number and heading level, or ErrHeadingNotFound.
func (d *Document) findHeading(heading string) (int, int, error) {
	lines := d.lines()
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for _, ch := range trimmed {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		text := strings.TrimSpace(trimmed[level:])
		if text == heading {
			return i + 1, level, nil
		}
	}
	return 0, 0, ErrHeadingNotFound
}

// findSectionEnd finds the end line of a section starting at lineNum with the given level.
// The section ends before the next heading of the same or higher level, or at EOF.
func (d *Document) findSectionEnd(lineNum int, level int) int {
	lines := d.lines()
	for i := lineNum; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		hLevel := 0
		for _, ch := range trimmed {
			if ch == '#' {
				hLevel++
			} else {
				break
			}
		}
		if hLevel <= level {
			return i // 0-based, which is the line before (1-based = i)
		}
	}
	return len(lines)
}

// GetSection returns content under a heading (up to next same-level heading),
// plus the start and end line numbers (1-based).
func (d *Document) GetSection(heading string) (string, int, int, error) {
	lineNum, level, err := d.findHeading(heading)
	if err != nil {
		return "", 0, 0, err
	}
	endLine := d.findSectionEnd(lineNum, level)
	lines := d.lines()
	section := lines[lineNum-1 : endLine]
	return strings.Join(section, "\n"), lineNum, endLine, nil
}

// InsertAfterHeading inserts content immediately after a heading line.
func (d *Document) InsertAfterHeading(heading string, content string) error {
	lineNum, _, err := d.findHeading(heading)
	if err != nil {
		return err
	}
	return d.InsertLines(lineNum+1, content)
}

// MoveSection moves an entire section (heading + body) to after a target heading.
func (d *Document) MoveSection(heading string, targetHeading string) error {
	// Find the source section
	srcLine, srcLevel, err := d.findHeading(heading)
	if err != nil {
		return err
	}
	srcEnd := d.findSectionEnd(srcLine, srcLevel)

	// Find the target heading
	tgtLine, tgtLevel, err := d.findHeading(targetHeading)
	if err != nil {
		return err
	}
	tgtEnd := d.findSectionEnd(tgtLine, tgtLevel)

	// Move the source block to after the target section
	return d.MoveLines(srcLine, srcEnd, tgtEnd+1)
}

// DeleteSection removes an entire section (heading + body).
func (d *Document) DeleteSection(heading string) error {
	lineNum, level, err := d.findHeading(heading)
	if err != nil {
		return err
	}
	endLine := d.findSectionEnd(lineNum, level)
	return d.DeleteLines(lineNum, endLine)
}

// ShiftHeadings shifts heading levels in a range.
// Positive levels demote (## → ###), negative levels promote (### → ##).
func (d *Document) ShiftHeadings(startLine int, endLine int, levels int) error {
	if err := d.validateLineRange(startLine, endLine); err != nil {
		return err
	}
	lines := d.lines()
	for i := startLine - 1; i < endLine; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		hLevel := 0
		for _, ch := range trimmed {
			if ch == '#' {
				hLevel++
			} else {
				break
			}
		}
		newLevel := hLevel + levels
		if newLevel < 1 {
			newLevel = 1
		}
		if newLevel > 6 {
			newLevel = 6
		}
		rest := trimmed[hLevel:]
		indent := lines[i][:len(lines[i])-len(trimmed)]
		lines[i] = indent + strings.Repeat("#", newLevel) + rest
	}
	d.setLines(lines)
	return nil
}

// InsertTableRow inserts a row into a markdown table, auto-formatting with pipes.
func (d *Document) InsertTableRow(lineNum int, values []string) error {
	lines := d.lines()
	if lineNum < 1 || lineNum > len(lines)+1 {
		return ErrLineOutOfRange
	}

	// Find the table context to determine column widths
	// Look for a nearby table row to match formatting
	var refLine string
	if lineNum > 1 && lineNum-2 < len(lines) && strings.Contains(lines[lineNum-2], "|") {
		refLine = lines[lineNum-2]
	} else if lineNum <= len(lines) && strings.Contains(lines[lineNum-1], "|") {
		refLine = lines[lineNum-1]
	}

	if refLine == "" {
		return ErrNotATable
	}

	// Parse column widths from the reference line
	refCols := parseTableRow(refLine)
	colWidths := make([]int, len(refCols))
	for i, col := range refCols {
		colWidths[i] = displayWidth(col)
	}

	// Build the new row, padding to match column widths
	row := "|"
	for i, v := range values {
		width := displayWidth(v)
		if i < len(colWidths) && colWidths[i] > width {
			width = colWidths[i]
		}
		row += " " + v + strings.Repeat(" ", width-displayWidth(v)) + " |"
	}

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:lineNum-1]...)
	newLines = append(newLines, row)
	newLines = append(newLines, lines[lineNum-1:]...)
	d.setLines(newLines)
	return nil
}

// UpdateTableRow replaces cell values at an existing table row line.
func (d *Document) UpdateTableRow(lineNum int, values []string) error {
	lines := d.lines()
	if lineNum < 1 || lineNum > len(lines) {
		return ErrLineOutOfRange
	}
	if !strings.Contains(lines[lineNum-1], "|") {
		return ErrNotATable
	}

	// Find a reference line for column widths (check header or adjacent row)
	var refLine string
	for i := lineNum - 2; i >= 0; i-- {
		if strings.Contains(lines[i], "|") {
			refLine = lines[i]
			break
		}
	}
	if refLine == "" {
		refLine = lines[lineNum-1]
	}

	refCols := parseTableRow(refLine)
	colWidths := make([]int, len(refCols))
	for i, col := range refCols {
		colWidths[i] = displayWidth(col)
	}

	row := "|"
	for i, v := range values {
		width := displayWidth(v)
		if i < len(colWidths) && colWidths[i] > width {
			width = colWidths[i]
		}
		row += " " + v + strings.Repeat(" ", width-displayWidth(v)) + " |"
	}

	lines[lineNum-1] = row
	d.setLines(lines)
	return nil
}

// DeleteTableRow removes a table row at the given line number.
func (d *Document) DeleteTableRow(lineNum int) error {
	lines := d.lines()
	if lineNum < 1 || lineNum > len(lines) {
		return ErrLineOutOfRange
	}
	if !strings.Contains(lines[lineNum-1], "|") {
		return ErrNotATable
	}

	newLines := make([]string, 0, len(lines)-1)
	newLines = append(newLines, lines[:lineNum-1]...)
	newLines = append(newLines, lines[lineNum:]...)
	d.setLines(newLines)
	return nil
}

// CheckboxItem represents a single markdown checkbox line.
type CheckboxItem struct {
	Line    int    `json:"line"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

// CheckboxGroup represents checkboxes grouped under a heading.
type CheckboxGroup struct {
	Heading string         `json:"heading"`
	Level   int            `json:"level"`
	Line    int            `json:"line"`
	Items   []CheckboxItem `json:"items"`
}

// GetCheckboxes extracts markdown checkboxes, optionally filtered and ranged.
// filter: "all", "checked", or "unchecked". startLine/endLine of 0 means full document.
func (d *Document) GetCheckboxes(filter string, startLine int, endLine int) ([]CheckboxItem, error) {
	startLine, endLine, err := d.resolveLineRange(startLine, endLine)
	if err != nil {
		return nil, err
	}
	if d.GetLineCount() == 0 {
		return []CheckboxItem{}, nil
	}

	lines := d.lines()
	var items []CheckboxItem
	for i := startLine - 1; i < endLine; i++ {
		trimmed := strings.TrimSpace(lines[i])
		isChecked := strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]")
		isUnchecked := strings.HasPrefix(trimmed, "- [ ]")
		if !isChecked && !isUnchecked {
			continue
		}

		if filter == "checked" && !isChecked {
			continue
		}
		if filter == "unchecked" && !isUnchecked {
			continue
		}

		text := trimmed
		if isChecked {
			text = strings.TrimSpace(trimmed[5:])
		} else {
			text = strings.TrimSpace(trimmed[5:])
		}

		items = append(items, CheckboxItem{
			Line:    i + 1,
			Text:    text,
			Checked: isChecked,
		})
	}
	if items == nil {
		items = []CheckboxItem{}
	}
	return items, nil
}

// GetCheckboxTree extracts checkboxes grouped under their nearest parent heading.
func (d *Document) GetCheckboxTree(filter string, startLine int, endLine int) ([]CheckboxGroup, error) {
	items, err := d.GetCheckboxes(filter, startLine, endLine)
	if err != nil {
		return nil, err
	}

	toc, err := d.GetTableOfContents()
	if err != nil {
		return nil, err
	}

	// Build groups: assign each checkbox to its nearest preceding heading
	var groups []CheckboxGroup
	groupMap := make(map[int]*CheckboxGroup) // heading line -> group

	for _, item := range items {
		// Find nearest heading before this checkbox
		var bestHeading *HeadingInfo
		for i := range toc {
			if toc[i].LineNumber < item.Line {
				bestHeading = &toc[i]
			} else {
				break
			}
		}

		if bestHeading == nil {
			// No heading before this checkbox — put in a root group
			key := 0
			if _, ok := groupMap[key]; !ok {
				group := CheckboxGroup{
					Heading: "(top-level)",
					Level:   0,
					Line:    0,
					Items:   []CheckboxItem{},
				}
				groups = append(groups, group)
				groupMap[key] = &groups[len(groups)-1]
			}
			groupMap[key].Items = append(groupMap[key].Items, item)
		} else {
			key := bestHeading.LineNumber
			if _, ok := groupMap[key]; !ok {
				group := CheckboxGroup{
					Heading: bestHeading.Heading,
					Level:   bestHeading.Level,
					Line:    bestHeading.LineNumber,
					Items:   []CheckboxItem{},
				}
				groups = append(groups, group)
				groupMap[key] = &groups[len(groups)-1]
			}
			groupMap[key].Items = append(groupMap[key].Items, item)
		}
	}

	if groups == nil {
		groups = []CheckboxGroup{}
	}
	return groups, nil
}

// isMarkdown returns true if the document path has a markdown extension.
func (d *Document) isMarkdown() bool {
	ext := strings.ToLower(filepath.Ext(d.Path))
	return ext == ".md" || ext == ".mdown" || ext == ".markdown" || ext == ".mkd" || ext == ".mdwn" || ext == ".mdx"
}

// Format normalizes formatting within a line range.
// startLine/endLine of 0 means full document.
// All files: trailing whitespace removal, consecutive blank line collapsing.
// Markdown files only: table alignment, heading spacing.
func (d *Document) Format(startLine int, endLine int) error {
	startLine, endLine, err := d.resolveLineRange(startLine, endLine)
	if err != nil {
		return err
	}
	if d.GetLineCount() == 0 {
		return nil
	}

	lines := d.lines()

	// Process only the target range
	section := make([]string, endLine-startLine+1)
	copy(section, lines[startLine-1:endLine])

	// 1. Remove trailing whitespace
	for i := range section {
		section[i] = strings.TrimRight(section[i], " \t")
	}

	// 2. Collapse consecutive blank lines to one
	collapsed := make([]string, 0, len(section))
	prevBlank := false
	for _, line := range section {
		isBlank := strings.TrimSpace(line) == ""
		if isBlank && prevBlank {
			continue
		}
		collapsed = append(collapsed, line)
		prevBlank = isBlank
	}
	section = collapsed

	// Markdown-only rules
	if d.isMarkdown() {
		// 3. Ensure blank line before headings (unless first line of section)
		formatted := make([]string, 0, len(section)+10)
		for i, line := range section {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") && i > 0 {
				prevTrimmed := strings.TrimSpace(section[i-1])
				if prevTrimmed != "" {
					formatted = append(formatted, "")
				}
			}
			formatted = append(formatted, line)
		}
		section = formatted

		// 4. Table alignment
		section = formatTables(section)
	}

	// Stitch back
	result := make([]string, 0, len(lines)-endLine+startLine-1+len(section))
	result = append(result, lines[:startLine-1]...)
	result = append(result, section...)
	result = append(result, lines[endLine:]...)
	d.setLines(result)
	return nil
}

// formatTables finds markdown tables in lines and aligns their columns.
func formatTables(lines []string) []string {
	result := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		// Detect table start: look for separator row (|---|)
		if isTableSeparator(lines[i]) || (i+1 < len(lines) && isTableSeparator(lines[i+1])) {
			// Find table boundaries
			tableStart := i
			// Walk back to find header
			if i > 0 && isTableRow(lines[i-1]) {
				// Already added the header line to result, remove it to reformat
				tableStart = len(result) - 1
				result = result[:tableStart]
				i = i - 1
			}
			// Collect all table rows
			var tableRows []string
			for i < len(lines) && (isTableRow(lines[i]) || isTableSeparator(lines[i])) {
				tableRows = append(tableRows, lines[i])
				i++
			}
			// Align the table
			aligned := alignTable(tableRows)
			result = append(result, aligned...)
		} else {
			result = append(result, lines[i])
			i++
		}
	}
	return result
}

func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	// Must contain at least one ---
	return strings.Contains(trimmed, "---")
}

func alignTable(rows []string) []string {
	if len(rows) == 0 {
		return rows
	}

	// Parse all rows into cells
	parsed := make([][]string, len(rows))
	maxCols := 0
	separatorIdx := -1
	for i, row := range rows {
		parsed[i] = parseTableRow(row)
		if len(parsed[i]) > maxCols {
			maxCols = len(parsed[i])
		}
		if separatorIdx == -1 && isTableSeparator(row) {
			separatorIdx = i
		}
	}

	// Find max width per column
	colWidths := make([]int, maxCols)
	for i, cells := range parsed {
		if i == separatorIdx {
			continue // Don't count separator dashes
		}
		for j, cell := range cells {
			if j < maxCols && displayWidth(cell) > colWidths[j] {
				colWidths[j] = displayWidth(cell)
			}
		}
	}
	// Minimum width of 3 for columns
	for i := range colWidths {
		if colWidths[i] < 3 {
			colWidths[i] = 3
		}
	}

	// Rebuild rows
	result := make([]string, len(rows))
	for i, cells := range parsed {
		row := "|"
		for j := 0; j < maxCols; j++ {
			cell := ""
			if j < len(cells) {
				cell = cells[j]
			}
			if i == separatorIdx {
				// Separator row: fill with dashes
				row += " " + strings.Repeat("-", colWidths[j]) + " |"
			} else {
				row += " " + cell + strings.Repeat(" ", colWidths[j]-displayWidth(cell)) + " |"
			}
		}
		result[i] = row
	}
	return result
}

// parseTableRow extracts cell values from a markdown table row.
func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}
	parts := strings.Split(line, "|")
	cols := make([]string, len(parts))
	for i, p := range parts {
		cols[i] = strings.TrimSpace(p)
	}
	return cols
}

// Append appends content to the end of the document.
func (d *Document) Append(content string) error {
	if d.Content == "" {
		d.Content = content
	} else {
		d.Content = d.Content + "\n" + content
	}
	return nil
}
