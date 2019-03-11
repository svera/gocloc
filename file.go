package gocloc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const tabToSpaces = "    "
const logicIndentSize = 4

type ClocFile struct {
	Code       int32   `xml:"code,attr" json:"code"`
	Comments   int32   `xml:"comment,attr" json:"comment"`
	Blanks     int32   `xml:"blank,attr" json:"blank"`
	Complexity float32 `xml:"complexity,attr" json:"complexity"`
	Name       string  `xml:"name,attr" json:"name"`
	Lang       string  `xml:"language,attr" json:"language"`
}

type ClocFiles []ClocFile

func (cf ClocFiles) Len() int {
	return len(cf)
}
func (cf ClocFiles) Swap(i, j int) {
	cf[i], cf[j] = cf[j], cf[i]
}
func (cf ClocFiles) Less(i, j int) bool {
	if cf[i].Code == cf[j].Code {
		return cf[i].Name < cf[j].Name
	}
	return cf[i].Code > cf[j].Code
}

func AnalyzeFile(filename string, language *Language, opts *ClocOptions) *ClocFile {
	fp, err := os.Open(filename)
	if err != nil {
		// ignore error
		return &ClocFile{Name: filename}
	}
	defer fp.Close()

	return AnalyzeReader(filename, language, fp, opts)
}

func AnalyzeReader(filename string, language *Language, file io.Reader, opts *ClocOptions) *ClocFile {
	if opts.Debug {
		fmt.Printf("filename=%v\n", filename)
	}

	clocFile := &ClocFile{
		Name: filename,
	}

	isFirstLine := true
	isInComments := false
	isInCommentsSame := false
	var indents int32
	buf := getByteSlice()
	defer putByteSlice(buf)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lineOrg := scanner.Text()
		lineOrg = strings.Replace(lineOrg, "\t", tabToSpaces, -1)
		line := strings.TrimSpace(lineOrg)

		if len(strings.TrimSpace(line)) == 0 {
			clocFile.Blanks++
			if opts.Debug {
				fmt.Printf("[BLNK,cd:%d,cm:%d,bk:%d,iscm:%v] %s\n",
					clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineOrg)
			}
			continue
		}

		// shebang line is 'code'
		if isFirstLine && strings.HasPrefix(line, "#!") {
			clocFile.Code++
			isFirstLine = false
			if opts.Debug {
				fmt.Printf("[CODE,cd:%d,cm:%d,bk:%d,iscm:%v] %s\n",
					clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineOrg)
			}
			continue
		}

		if len(language.lineComments) > 0 {
			isSingleComment := false
			if isFirstLine {
				line = trimBOM(line)
			}
			for _, singleComment := range language.lineComments {
				if strings.HasPrefix(line, singleComment) {
					clocFile.Comments++
					isSingleComment = true
					break
				}
			}
			if isSingleComment {
				if opts.Debug {
					fmt.Printf("[COMM,cd:%d,cm:%d,bk:%d,iscm:%v] %s\n",
						clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineOrg)
				}
				continue
			}
		}

		isCode := false
		multiLine := ""
		multiLineEnd := ""
		for i := range language.multiLines {
			multiLine = language.multiLines[i][0]
			multiLineEnd = language.multiLines[i][1]
			if multiLine != "" {
				if strings.HasPrefix(line, multiLine) {
					isInComments = true
				} else if strings.HasSuffix(line, multiLineEnd) {
					isInComments = true
				} else if containComments(line, multiLine, multiLineEnd) {
					isInComments = true
					if (multiLine != multiLineEnd) &&
						(strings.HasSuffix(line, multiLine) || strings.HasPrefix(line, multiLineEnd)) {
						clocFile.Code++
						isCode = true
						if opts.Debug {
							fmt.Printf("[CODE,cd:%d,cm:%d,bk:%d,iscm:%v] %s\n",
								clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineOrg)
						}
						continue
					}
				}
				if isInComments {
					break
				}
			}
		}

		if isInComments && isCode {
			continue
		}

		if isInComments {
			if multiLine == multiLineEnd {
				if strings.Count(line, multiLineEnd) == 2 {
					isInComments = false
					isInCommentsSame = false
				} else if strings.HasPrefix(line, multiLineEnd) ||
					strings.HasSuffix(line, multiLineEnd) {
					if isInCommentsSame {
						isInComments = false
					}
					isInCommentsSame = !isInCommentsSame
				}
			} else {
				if strings.Contains(line, multiLineEnd) {
					isInComments = false
				}
			}
			clocFile.Comments++
			if opts.Debug {
				fmt.Printf("[COMM,cd:%d,cm:%d,bk:%d,iscm:%v,iscms:%v] %s\n",
					clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, isInCommentsSame, lineOrg)
			}
			continue
		}

		indents += int32(len(lineOrg)) - int32(len(line))/logicIndentSize
		clocFile.Code++
		if opts.Debug {
			fmt.Printf("[CODE,cd:%d,cm:%d,bk:%d,iscm:%v] %s\n",
				clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineOrg)
		}
	}

	clocFile.Complexity = float32(indents) / float32(clocFile.Code)
	return clocFile
}
