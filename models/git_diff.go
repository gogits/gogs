// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/Unknwon/com"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"

	"github.com/gogits/git-module"

	"github.com/gogits/gogs/modules/base"
	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/process"
	"github.com/gogits/gogs/modules/template/highlight"
)

type DiffLineType uint8

const (
	DIFF_LINE_PLAIN DiffLineType = iota + 1
	DIFF_LINE_ADD
	DIFF_LINE_DEL
	DIFF_LINE_SECTION
)

type DiffFileType uint8

const (
	DIFF_FILE_ADD DiffFileType = iota + 1
	DIFF_FILE_CHANGE
	DIFF_FILE_DEL
	DIFF_FILE_RENAME
)

type DiffLine struct {
	LeftIdx  int
	RightIdx int
	Type     DiffLineType
	Content  string
}

func (d *DiffLine) GetType() int {
	return int(d.Type)
}

type DiffSection struct {
	Name  string
	Lines []*DiffLine
}

var (
	addedCodePrefix   = []byte("<span class=\"added-code\">")
	removedCodePrefix = []byte("<span class=\"removed-code\">")
	codeTagSuffix     = []byte("</span>")
)

func diffToHTML(diffs []diffmatchpatch.Diff, lineType DiffLineType) template.HTML {
	var buf bytes.Buffer
	for i := range diffs {
		if diffs[i].Type == diffmatchpatch.DiffInsert && lineType == DIFF_LINE_ADD {
			buf.Write(addedCodePrefix)
			buf.WriteString(html.EscapeString(diffs[i].Text))
			buf.Write(codeTagSuffix)
		} else if diffs[i].Type == diffmatchpatch.DiffDelete && lineType == DIFF_LINE_DEL {
			buf.Write(removedCodePrefix)
			buf.WriteString(html.EscapeString(diffs[i].Text))
			buf.Write(codeTagSuffix)
		} else if diffs[i].Type == diffmatchpatch.DiffEqual {
			buf.WriteString(html.EscapeString(diffs[i].Text))
		}
	}

	return template.HTML(buf.Bytes())
}

// get an specific line by type (add or del) and file line number
func (diffSection *DiffSection) GetLine(lineType DiffLineType, idx int) *DiffLine {
	difference := 0

	for _, diffLine := range diffSection.Lines {
		if diffLine.Type == DIFF_LINE_PLAIN {
			// get the difference of line numbers between ADD and DEL versions
			difference = diffLine.RightIdx - diffLine.LeftIdx
			continue
		}

		if lineType == DIFF_LINE_DEL {
			if diffLine.RightIdx == 0 && diffLine.LeftIdx == idx-difference {
				return diffLine
			}
		} else if lineType == DIFF_LINE_ADD {
			if diffLine.LeftIdx == 0 && diffLine.RightIdx == idx+difference {
				return diffLine
			}
		}
	}
	return nil
}

// computes inline diff for the given line
func (diffSection *DiffSection) GetComputedInlineDiffFor(diffLine *DiffLine) template.HTML {
	var compareDiffLine *DiffLine
	var diff1, diff2 string

	getDefaultReturn := func() template.HTML {
		return template.HTML(html.EscapeString(diffLine.Content[1:]))
	}

	// just compute diff for adds and removes
	if diffLine.Type != DIFF_LINE_ADD && diffLine.Type != DIFF_LINE_DEL {
		return getDefaultReturn()
	}

	// try to find equivalent diff line. ignore, otherwise
	if diffLine.Type == DIFF_LINE_ADD {
		compareDiffLine = diffSection.GetLine(DIFF_LINE_DEL, diffLine.RightIdx)
		if compareDiffLine == nil {
			return getDefaultReturn()
		}
		diff1 = compareDiffLine.Content
		diff2 = diffLine.Content
	} else {
		compareDiffLine = diffSection.GetLine(DIFF_LINE_ADD, diffLine.LeftIdx)
		if compareDiffLine == nil {
			return getDefaultReturn()
		}
		diff1 = diffLine.Content
		diff2 = compareDiffLine.Content
	}

	dmp := diffmatchpatch.New()
	diffRecord := dmp.DiffMain(diff1[1:], diff2[1:], true)
	diffRecord = dmp.DiffCleanupSemantic(diffRecord)

	return diffToHTML(diffRecord, diffLine.Type)
}

type DiffFile struct {
	Name               string
	OldName            string
	Index              int
	Addition, Deletion int
	Type               DiffFileType
	IsCreated          bool
	IsDeleted          bool
	IsBin              bool
	IsRenamed          bool
	Sections           []*DiffSection
	IsIncomplete       bool
}

func (diffFile *DiffFile) GetType() int {
	return int(diffFile.Type)
}

func (diffFile *DiffFile) GetHighlightClass() string {
	return highlight.FileNameToHighlightClass(diffFile.Name)
}

type Diff struct {
	TotalAddition, TotalDeletion int
	Files                        []*DiffFile
	IsIncomplete                 bool
}

func (diff *Diff) NumFiles() int {
	return len(diff.Files)
}

const DIFF_HEAD = "diff --git "

func ParsePatch(maxLines, maxLineCharacteres, maxFiles int, reader io.Reader) (*Diff, error) {
	var (
		diff = &Diff{Files: make([]*DiffFile, 0)}

		curFile    *DiffFile
		curSection = &DiffSection{
			Lines: make([]*DiffLine, 0, 10),
		}

		leftLine, rightLine int
		lineCount           int
		curFileLinesCount   int
	)

	input := bufio.NewReader(reader)
	isEOF := false
	for !isEOF {
		line, err := input.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				isEOF = true
			} else {
				return nil, fmt.Errorf("ReadString: %v", err)
			}
		}

		if len(line) > 0 && line[len(line)-1] == '\n' {
			// Remove line break.
			line = line[:len(line)-1]
		}

		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") || len(line) == 0 {
			continue
		}

		curFileLinesCount++
		lineCount++

		// Diff data too large, we only show the first about maxlines lines
		if curFileLinesCount >= maxLines || len(line) >= maxLineCharacteres {
			curFile.IsIncomplete = true
		}

		switch {
		case line[0] == ' ':
			diffLine := &DiffLine{Type: DIFF_LINE_PLAIN, Content: line, LeftIdx: leftLine, RightIdx: rightLine}
			leftLine++
			rightLine++
			curSection.Lines = append(curSection.Lines, diffLine)
			continue
		case line[0] == '@':
			curSection = &DiffSection{}
			curFile.Sections = append(curFile.Sections, curSection)
			ss := strings.Split(line, "@@")
			diffLine := &DiffLine{Type: DIFF_LINE_SECTION, Content: line}
			curSection.Lines = append(curSection.Lines, diffLine)

			// Parse line number.
			ranges := strings.Split(ss[1][1:], " ")
			leftLine, _ = com.StrTo(strings.Split(ranges[0], ",")[0][1:]).Int()
			if len(ranges) > 1 {
				rightLine, _ = com.StrTo(strings.Split(ranges[1], ",")[0]).Int()
			} else {
				log.Warn("Parse line number failed: %v", line)
				rightLine = leftLine
			}
			continue
		case line[0] == '+':
			curFile.Addition++
			diff.TotalAddition++
			diffLine := &DiffLine{Type: DIFF_LINE_ADD, Content: line, RightIdx: rightLine}
			rightLine++
			curSection.Lines = append(curSection.Lines, diffLine)
			continue
		case line[0] == '-':
			curFile.Deletion++
			diff.TotalDeletion++
			diffLine := &DiffLine{Type: DIFF_LINE_DEL, Content: line, LeftIdx: leftLine}
			if leftLine > 0 {
				leftLine++
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		case strings.HasPrefix(line, "Binary"):
			curFile.IsBin = true
			continue
		}

		// Get new file.
		if strings.HasPrefix(line, DIFF_HEAD) {
			middle := -1

			// Note: In case file name is surrounded by double quotes (it happens only in git-shell).
			// e.g. diff --git "a/xxx" "b/xxx"
			hasQuote := line[len(DIFF_HEAD)] == '"'
			if hasQuote {
				middle = strings.Index(line, ` "b/`)
			} else {
				middle = strings.Index(line, " b/")
			}

			beg := len(DIFF_HEAD)
			a := line[beg+2 : middle]
			b := line[middle+3:]
			if hasQuote {
				a = string(git.UnescapeChars([]byte(a[1 : len(a)-1])))
				b = string(git.UnescapeChars([]byte(b[1 : len(b)-1])))
			}

			curFile = &DiffFile{
				Name:     a,
				Index:    len(diff.Files) + 1,
				Type:     DIFF_FILE_CHANGE,
				Sections: make([]*DiffSection, 0, 10),
			}
			diff.Files = append(diff.Files, curFile)
			if len(diff.Files) >= maxFiles {
				diff.IsIncomplete = true
				io.Copy(ioutil.Discard, reader)
				break
			}
			curFileLinesCount = 0

			// Check file diff type.
			for {
				line, err := input.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						isEOF = true
					} else {
						return nil, fmt.Errorf("ReadString: %v", err)
					}
				}

				switch {
				case strings.HasPrefix(line, "new file"):
					curFile.Type = DIFF_FILE_ADD
					curFile.IsCreated = true
				case strings.HasPrefix(line, "deleted"):
					curFile.Type = DIFF_FILE_DEL
					curFile.IsDeleted = true
				case strings.HasPrefix(line, "index"):
					curFile.Type = DIFF_FILE_CHANGE
				case strings.HasPrefix(line, "similarity index 100%"):
					curFile.Type = DIFF_FILE_RENAME
					curFile.IsRenamed = true
					curFile.OldName = curFile.Name
					curFile.Name = b
				}
				if curFile.Type > 0 {
					break
				}
			}
		}
	}

	// FIXME: detect encoding while parsing.
	var buf bytes.Buffer
	for _, f := range diff.Files {
		buf.Reset()
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				buf.WriteString(l.Content)
				buf.WriteString("\n")
			}
		}
		charsetLabel, err := base.DetectEncoding(buf.Bytes())
		if charsetLabel != "UTF-8" && err == nil {
			encoding, _ := charset.Lookup(charsetLabel)
			if encoding != nil {
				d := encoding.NewDecoder()
				for _, sec := range f.Sections {
					for _, l := range sec.Lines {
						if c, _, err := transform.String(d, l.Content); err == nil {
							l.Content = c
						}
					}
				}
			}
		}
	}
	return diff, nil
}

func GetDiffRange(repoPath, beforeCommitID string, afterCommitID string, maxLines, maxLineCharacteres, maxFiles int) (*Diff, error) {
	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}

	commit, err := repo.GetCommit(afterCommitID)
	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	// if "after" commit given
	if len(beforeCommitID) == 0 {
		// First commit of repository.
		if commit.ParentCount() == 0 {
			cmd = exec.Command("git", "show", afterCommitID)
		} else {
			c, _ := commit.Parent(0)
			cmd = exec.Command("git", "diff", "-M", c.ID.String(), afterCommitID)
		}
	} else {
		cmd = exec.Command("git", "diff", "-M", beforeCommitID, afterCommitID)
	}
	cmd.Dir = repoPath
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	pid := process.Add(fmt.Sprintf("GetDiffRange (%s)", repoPath), cmd)
	defer process.Remove(pid)

	diff, err := ParsePatch(maxLines, maxLineCharacteres, maxFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	return diff, nil
}

func GetRawDiff(repoPath, commitID, diffType string) (string, error) {
	repo, err := git.OpenRepository(repoPath)
	if err != nil {
		return "", err
	}

	commit, err := repo.GetCommit(commitID)
	if err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	switch diffType {
	case "diff":
		if commit.ParentCount() == 0 {
			cmd = exec.Command("git", "show", commitID)
		} else {
			c, _ := commit.Parent(0)
			cmd = exec.Command("git", "diff", "-M", c.ID.String(), commitID)
		}
	case "patch":
		if commit.ParentCount() == 0 {
			cmd = exec.Command("git", "format-patch", "--no-signature", "--stdout", "--root", commitID)
		} else {
			c, _ := commit.Parent(0)
			query := fmt.Sprintf("%s...%s", commitID, c.ID.String())
			cmd = exec.Command("git", "format-patch", "--no-signature", "--stdout", query)
		}
	default:
		return "", fmt.Errorf("Invalid diffType '%s'", diffType)
	}

	stderr := new(bytes.Buffer)

	cmd.Dir = repoPath
	cmd.Stderr = stderr

	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Stdout: %v; Stderr: %s", err, stderr.String())
	}
	return string(stdout), nil
}

func GetDiffCommit(repoPath, commitID string, maxLines, maxLineCharacteres, maxFiles int) (*Diff, error) {
	return GetDiffRange(repoPath, "", commitID, maxLines, maxLineCharacteres, maxFiles)
}
