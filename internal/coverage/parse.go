package coverage

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// record is "file:SL.SC,EL.EC statements count", the whole line.
var record = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

var modes = map[string]bool{"set": true, "count": true, "atomic": true}

// parse adds data's blocks to p and returns how many it read. Each profile
// must start with a mode line; concatenated profiles may repeat it.
func (p *Profile) parse(data []byte, index map[string]map[[4]int]int) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	read, header := 0, false
	for line := 1; scanner.Scan(); line++ {
		isBlock, err := p.parseLine(strings.TrimRight(scanner.Text(), "\r"), &header, index)
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", line, err)
		}
		if isBlock {
			read++
		}
	}
	return read, scanner.Err()
}

// parseLine handles one line: blank, a mode header, or a block record.
func (p *Profile) parseLine(text string, header *bool, index map[string]map[[4]int]int) (bool, error) {
	if strings.TrimSpace(text) == "" {
		return false, nil
	}
	if mode, ok := strings.CutPrefix(text, "mode: "); ok {
		if !modes[mode] {
			return false, fmt.Errorf("unknown coverage mode %q (want set, count or atomic)", mode)
		}
		*header = true
		return false, nil
	}
	if !*header {
		return false, errors.New(`missing "mode:" header; not a go coverage profile`)
	}
	file, b, err := parseRecord(text)
	if err != nil {
		return false, err
	}
	return true, p.add(file, b, index)
}

// parseRecord parses a block record and checks that its coordinates are
// positive and ordered.
func parseRecord(text string) (string, Block, error) {
	var b Block
	m := record.FindStringSubmatch(text)
	if m == nil {
		return "", b, fmt.Errorf("invalid coverage record %q", text)
	}
	fields := []*int{&b.StartLine, &b.StartCol, &b.EndLine, &b.EndCol, &b.Statements, &b.Count}
	for i, f := range fields {
		v, err := strconv.Atoi(m[i+2])
		if err != nil {
			return "", b, fmt.Errorf("invalid number %q in %q", m[i+2], text)
		}
		*f = v
	}
	if b.StartLine < 1 || b.StartCol < 1 || b.EndLine < 1 || b.EndCol < 1 {
		return "", b, fmt.Errorf("line and column numbers start at 1 in %q", text)
	}
	if b.EndLine < b.StartLine || b.EndLine == b.StartLine && b.EndCol < b.StartCol {
		return "", b, fmt.Errorf("span ends before it starts in %q", text)
	}
	return m[1], b, nil
}

// add records b, merging a repeated span by keeping the highest count. A
// repeated span with a different statement count is from different source.
func (p *Profile) add(file string, b Block, index map[string]map[[4]int]int) error {
	spans := index[file]
	if spans == nil {
		spans = map[[4]int]int{}
		index[file] = spans
	}
	key := [4]int{b.StartLine, b.StartCol, b.EndLine, b.EndCol}
	i, seen := spans[key]
	if !seen {
		spans[key] = len(p.Files[file])
		p.Files[file] = append(p.Files[file], b)
		return nil
	}
	prev := &p.Files[file][i]
	if prev.Statements != b.Statements {
		return fmt.Errorf("%s:%d.%d,%d.%d has %d statements here and %d in an earlier record", file, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.Statements, prev.Statements)
	}
	prev.Count = max(prev.Count, b.Count)
	return nil
}
