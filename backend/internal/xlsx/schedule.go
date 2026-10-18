// Package xlsx writes the fixed two-sheet schedule workbook using inline text cells.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/reekta92/wad_project/backend/internal/store"
)

const MediaType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

func Schedule(exams []store.ScheduleExam) ([]byte, error) {
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	fixed := map[string]string{
		"[Content_Types].xml":        `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Calendar" sheetId="1" r:id="rId1"/><sheet name="List" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`,
	}
	// Fixed part ordering makes the archive reproducible.
	for _, path := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml"} {
		file, err := archive.Create(path)
		if err != nil {
			return nil, err
		}
		if _, err := file.Write([]byte(xml.Header + fixed[path])); err != nil {
			return nil, err
		}
	}
	list := [][]cell{{{1, "Exam ID"}, {2, "Course ID"}, {3, "Course code"}, {4, "Course name"}, {5, "Term"}, {6, "Department ID"}, {7, "Department"}, {8, "Session ID"}, {9, "Date (Europe/Istanbul)"}, {10, "Start time"}, {11, "End time"}, {12, "Rooms"}}}
	dates, times := map[string]bool{}, map[string]bool{}
	calendar := map[string]map[string][]string{}
	for _, exam := range exams {
		rooms := make([]string, 0, len(exam.Rooms))
		for _, room := range exam.Rooms {
			rooms = append(rooms, fmt.Sprintf("%s (floor %d)", room.Name, room.Floor))
		}
		roomText := strings.Join(rooms, ", ")
		list = append(list, []cell{{1, strconv.FormatInt(exam.ID, 10)}, {2, strconv.FormatInt(exam.CourseID, 10)}, {3, exam.CourseCode}, {4, exam.CourseName}, {5, strconv.Itoa(exam.Term)}, {6, strconv.FormatInt(exam.DepartmentID, 10)}, {7, exam.Department}, {8, strconv.FormatInt(exam.SessionID, 10)}, {9, exam.Date}, {10, exam.StartTime}, {11, exam.EndTime}, {12, roomText}})
		slot := exam.StartTime + "–" + exam.EndTime
		dates[exam.Date], times[slot] = true, true
		if calendar[slot] == nil {
			calendar[slot] = map[string][]string{}
		}
		calendar[slot][exam.Date] = append(calendar[slot][exam.Date], exam.CourseCode+" · "+exam.CourseName+"\n"+roomText)
	}
	dateKeys, timeKeys := keys(dates), keys(times)
	header := []cell{{1, "Session (Europe/Istanbul)"}}
	columns := map[string]int{}
	for i, date := range dateKeys {
		header = append(header, cell{i + 2, date})
		columns[date] = i + 2
	}
	grid := [][]cell{header}
	for _, slot := range timeKeys {
		row := []cell{{1, slot}}
		// Write only occupied calendar cells so sparse date/time combinations stay bounded.
		for date, entries := range calendar[slot] {
			row = append(row, cell{columns[date], strings.Join(entries, "\n\n")})
		}
		sort.Slice(row, func(i, j int) bool { return row[i].column < row[j].column })
		grid = append(grid, row)
	}
	for index, rows := range [][][]cell{grid, list} {
		document, err := worksheet(rows)
		if err != nil {
			return nil, err
		}
		file, err := archive.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", index+1))
		if err != nil {
			return nil, err
		}
		if _, err := file.Write(document); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

type cell struct {
	column int
	text   string
}

func keys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func columnName(number int) string {
	name := ""
	for number > 0 {
		number--
		name = string(rune('A'+number%26)) + name
		number /= 26
	}
	return name
}

func worksheet(rows [][]cell) ([]byte, error) {
	var result bytes.Buffer
	result.WriteString(xml.Header + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetFormatPr defaultRowHeight="30"/><cols><col min="1" max="16384" width="28" customWidth="1"/></cols><sheetData>`)
	for i, row := range rows {
		fmt.Fprintf(&result, `<row r="%d">`, i+1)
		for _, value := range row {
			if len(utf16.Encode([]rune(value.text))) > 32767 {
				return nil, store.ErrExportLimit
			}
			// Inline strings prevent spreadsheet formulas, including leading =, +, -, or @.
			fmt.Fprintf(&result, `<c r="%s%d" t="inlineStr" s="0"><is><t xml:space="preserve">`, columnName(value.column), i+1)
			text := spreadsheetText(value.text)
			if err := xml.EscapeText(&result, []byte(text)); err != nil {
				return nil, err
			}
			result.WriteString(`</t></is></c>`)
		}
		result.WriteString(`</row>`)
	}
	result.WriteString(`</sheetData></worksheet>`)
	return result.Bytes(), nil
}

// SpreadsheetML escapes control characters and literal escape-looking text separately from XML.
func spreadsheetText(value string) string {
	var result strings.Builder
	for i, r := range value {
		if r == '_' && len(value)-i >= 7 && value[i+1] == 'x' && value[i+6] == '_' {
			if _, err := strconv.ParseUint(value[i+2:i+6], 16, 16); err == nil {
				result.WriteString("_x005F_")
				continue
			}
		}
		if r < 32 && r != 9 && r != 10 || r == 0xFFFE || r == 0xFFFF {
			fmt.Fprintf(&result, "_x%04X_", r)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
