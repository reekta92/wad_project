package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/reekta92/wad_project/backend/internal/store"
)

func TestWorkbookContainsBothSheetsAndLiteralFilteredValues(t *testing.T) {
	exams := []store.ScheduleExam{{ID: 9223372036854775807, CourseCode: "=2+2", CourseName: "Öğrenci & <test>\x01_x0001_", Date: "2026-11-02", StartTime: "09:00:00", EndTime: "10:00:00"}}
	body, err := Schedule(exams)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := Schedule(exams)
	if err != nil || !bytes.Equal(body, repeated) {
		t.Fatal("archive is not deterministic")
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		decoder := xml.NewDecoder(bytes.NewReader(content))
		for {
			_, err = decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid %s: %v", file.Name, err)
			}
		}
		parts[file.Name] = string(content)
	}
	if len(parts) != 7 || !strings.Contains(parts["xl/workbook.xml"], `name="Calendar"`) || !strings.Contains(parts["xl/workbook.xml"], `name="List"`) {
		t.Fatal("missing workbook parts")
	}
	list := parts["xl/worksheets/sheet2.xml"]
	for _, value := range []string{"9223372036854775807", "=2+2", "Öğrenci &amp; &lt;test&gt;_x0001__x005F_x0001_", "2026-11-02"} {
		if !strings.Contains(list, value) {
			t.Errorf("missing %q: %s", value, list)
		}
	}
	if strings.Contains(list, "<f>") || !strings.Contains(parts["xl/worksheets/sheet1.xml"], "=2+2") {
		t.Fatal("unsafe formula or missing calendar data")
	}
}

func TestEmptyWorkbookAndCellLimit(t *testing.T) {
	if _, err := Schedule(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Schedule([]store.ScheduleExam{{CourseName: strings.Repeat("x", 32768)}}); err != store.ErrExportLimit {
		t.Fatal("oversized cell accepted")
	}
	if columnName(27) != "AA" || columnName(16384) != "XFD" {
		t.Fatal("invalid column addressing")
	}
}
