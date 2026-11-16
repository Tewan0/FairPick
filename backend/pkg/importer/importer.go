package importer

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type StudentInputRow struct {
	RowNumber   int    `json:"rowNumber"`
	StudentCode string `json:"studentCode"`
	FullName    string `json:"fullName"`
	RawContent  string `json:"rawContent"`
}

type InvalidRow struct {
	RowNumber  int    `json:"rowNumber"`
	RawContent string `json:"rawContent"`
	Reason     string `json:"reason"`
}

type ParseResult struct {
	TotalRowsRead int               `json:"totalRowsRead"`
	ValidRows     []StudentInputRow `json:"validRows"`
	InvalidRows   []InvalidRow      `json:"invalidRows"`
}

// ParseUploadedFile parses either a .csv or .xlsx file into validated student rows.
func ParseUploadedFile(filename string, content []byte) (*ParseResult, error) {
	if len(content) == 0 {
		return nil, errors.New("ไฟล์ที่อัปโหลดว่างเปล่า")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return ParseCSVBytes(content)
	case ".xlsx":
		return ParseXLSXBytes(content)
	default:
		return nil, fmt.Errorf("ไม่รองรับนามสกุลไฟล์ %s (รองรับเฉพาะ .xlsx และ .csv)", ext)
	}
}

// ParseCSVBytes parses UTF-8 CSV content (with or without BOM).
func ParseCSVBytes(content []byte) (*ParseResult, error) {
	// Strip UTF-8 BOM if present
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})

	text := strings.TrimSpace(string(content))
	if text == "" {
		return nil, errors.New("ไฟล์ CSV ไม่มีข้อมูล")
	}

	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	// Auto-detect tab or semicolon delimiter if first line uses them instead of comma
	firstLine := text
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		firstLine = text[:idx]
	}
	if !strings.Contains(firstLine, ",") {
		if strings.Contains(firstLine, "\t") {
			reader.Comma = '\t'
		} else if strings.Contains(firstLine, ";") {
			reader.Comma = ';'
		}
	}

	var rows [][]string
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("รูปแบบไฟล์ CSV ไม่ถูกต้อง: %w", err)
		}
		rows = append(rows, rec)
	}

	if len(rows) == 0 {
		return nil, errors.New("ไฟล์ CSV ไม่มีข้อมูล")
	}

	return analyzeGridRows(rows), nil
}

// ParseXLSXBytes parses an OpenXML .xlsx workbook from memory.
func ParseXLSXBytes(content []byte) (*ParseResult, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, errors.New("ไฟล์ .xlsx ไม่ถูกต้องหรือเสียหาย")
	}

	var sharedStrings []string
	var sheetFiles []*zip.File

	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if name == "xl/sharedStrings.xml" {
			ss, err := readSharedStrings(f)
			if err != nil {
				return nil, fmt.Errorf("ไม่สามารถอ่าน sharedStrings ในไฟล์ .xlsx: %w", err)
			}
			sharedStrings = ss
		} else if strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml") {
			sheetFiles = append(sheetFiles, f)
		}
	}

	if len(sheetFiles) == 0 {
		return nil, errors.New("ไม่พบ Worksheet ในไฟล์ .xlsx")
	}

	sort.Slice(sheetFiles, func(i, j int) bool {
		return sheetFiles[i].Name < sheetFiles[j].Name
	})

	rows, err := readWorksheetRows(sheetFiles[0], sharedStrings)
	if err != nil {
		return nil, fmt.Errorf("ไม่สามารถอ่านข้อมูลจาก Worksheet: %w", err)
	}

	if len(rows) == 0 {
		return nil, errors.New("ไฟล์ .xlsx ไม่มีข้อมูลใน Worksheet แรก")
	}

	return analyzeGridRows(rows), nil
}

type xlsxSST struct {
	SI []xlsxSI `xml:"si"`
}

type xlsxSI struct {
	T string  `xml:"t"`
	R []xlsxR `xml:"r"`
}

type xlsxR struct {
	T string `xml:"t"`
}

type xlsxWorksheet struct {
	SheetData struct {
		Rows []xlsxRow `xml:"row"`
	} `xml:"sheetData"`
}

type xlsxRow struct {
	R     int        `xml:"r,attr"`
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	R  string `xml:"r,attr"`
	T  string `xml:"t,attr"`
	V  string `xml:"v"`
	IS struct {
		T string  `xml:"t"`
		R []xlsxR `xml:"r"`
	} `xml:"is"`
}

const maxDecompressedXMLBytes = 15 * 1024 * 1024 // 15 MB max decompressed XML per entry to prevent Zip Bomb DoS

func readSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var sst xlsxSST
	if err := xml.NewDecoder(io.LimitReader(rc, maxDecompressedXMLBytes)).Decode(&sst); err != nil {
		return nil, err
	}

	res := make([]string, len(sst.SI))
	for i, item := range sst.SI {
		if len(item.R) > 0 {
			var sb strings.Builder
			for _, r := range item.R {
				sb.WriteString(r.T)
			}
			res[i] = sb.String()
		} else {
			res[i] = item.T
		}
	}
	return res, nil
}

func readWorksheetRows(f *zip.File, sharedStrings []string) ([][]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var ws xlsxWorksheet
	if err := xml.NewDecoder(io.LimitReader(rc, maxDecompressedXMLBytes)).Decode(&ws); err != nil {
		return nil, err
	}

	var rows [][]string
	for _, r := range ws.SheetData.Rows {
		if len(r.Cells) == 0 {
			continue
		}
		maxCol := 0
		colValues := make(map[int]string)
		for idx, c := range r.Cells {
			colIdx := parseCellColumnIndex(c.R, idx)
			if colIdx > maxCol {
				maxCol = colIdx
			}
			val := extractCellValue(c, sharedStrings)
			colValues[colIdx] = val
		}
		rowSlice := make([]string, maxCol+1)
		for colIdx, val := range colValues {
			rowSlice[colIdx] = val
		}
		rows = append(rows, rowSlice)
	}
	return rows, nil
}

func parseCellColumnIndex(cellRef string, fallback int) int {
	cellRef = strings.TrimSpace(cellRef)
	if cellRef == "" {
		return fallback
	}
	col := 0
	foundLetter := false
	for i := 0; i < len(cellRef); i++ {
		ch := cellRef[i]
		if ch >= 'A' && ch <= 'Z' {
			col = col*26 + int(ch-'A'+1)
			foundLetter = true
		} else if ch >= 'a' && ch <= 'z' {
			col = col*26 + int(ch-'a'+1)
			foundLetter = true
		} else {
			break
		}
	}
	if !foundLetter {
		return fallback
	}
	return col - 1
}

func extractCellValue(c xlsxCell, sharedStrings []string) string {
	switch c.T {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.V))
		if err == nil && idx >= 0 && idx < len(sharedStrings) {
			return sharedStrings[idx]
		}
		return c.V
	case "inlineStr":
		if len(c.IS.R) > 0 {
			var sb strings.Builder
			for _, r := range c.IS.R {
				sb.WriteString(r.T)
			}
			return sb.String()
		}
		return c.IS.T
	default:
		return c.V
	}
}

func analyzeGridRows(rows [][]string) *ParseResult {
	res := &ParseResult{
		ValidRows:   make([]StudentInputRow, 0),
		InvalidRows: make([]InvalidRow, 0),
	}

	codeCol := 0
	nameCol := 1
	lastNameCol := -1
	startRow := 0

	if len(rows) > 0 && isHeaderRow(rows[0]) {
		startRow = 1
		detectedCode, detectedName, detectedLast := detectHeaderColumns(rows[0])
		if detectedCode >= 0 {
			codeCol = detectedCode
		}
		if detectedName >= 0 {
			nameCol = detectedName
		}
		lastNameCol = detectedLast
	}

	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		if isRowCompletelyEmpty(row) {
			continue
		}

		res.TotalRowsRead++
		rawContent := strings.Join(row, ",")

		var studentCode, fullName string
		if codeCol < len(row) {
			studentCode = strings.TrimSpace(row[codeCol])
		}
		if nameCol < len(row) {
			fullName = strings.TrimSpace(row[nameCol])
		}
		if lastNameCol >= 0 && lastNameCol < len(row) {
			last := strings.TrimSpace(row[lastNameCol])
			if last != "" {
				fullName = strings.TrimSpace(fullName + " " + last)
			}
		} else if len(row) >= 3 && startRow == 0 && nameCol == 1 {
			// If no header and 3 columns (code, firstName, lastName), combine col 1 and col 2
			extra := strings.TrimSpace(row[2])
			if extra != "" && !strings.Contains(fullName, " ") {
				fullName = strings.TrimSpace(fullName + " " + extra)
			}
		}

		reason := validateStudentPair(studentCode, fullName)
		if reason != "" {
			res.InvalidRows = append(res.InvalidRows, InvalidRow{
				RowNumber:  rowNum,
				RawContent: rawContent,
				Reason:     reason,
			})
			continue
		}

		res.ValidRows = append(res.ValidRows, StudentInputRow{
			RowNumber:   rowNum,
			StudentCode: studentCode,
			FullName:    fullName,
			RawContent:  rawContent,
		})
	}

	return res
}

func isRowCompletelyEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func isHeaderRow(row []string) bool {
	for _, cell := range row {
		norm := strings.ToLower(strings.TrimSpace(cell))
		switch norm {
		case "student_code", "studentcode", "student id", "student_id", "code",
			"รหัสนักศึกษา", "รหัสประจำตัว", "รหัส", "full_name", "fullname", "name", "ชื่อ-นามสกุล", "ชื่อ - นามสกุล", "ชื่อ":
			return true
		}
	}
	return false
}

func detectHeaderColumns(header []string) (codeCol int, nameCol int, lastNameCol int) {
	codeCol = -1
	nameCol = -1
	lastNameCol = -1

	for i, cell := range header {
		norm := strings.ToLower(strings.TrimSpace(cell))
		switch norm {
		case "student_code", "studentcode", "student id", "student_id", "code", "รหัสนักศึกษา", "รหัสประจำตัว", "รหัส":
			if codeCol == -1 {
				codeCol = i
			}
		case "full_name", "fullname", "name", "ชื่อ-นามสกุล", "ชื่อ - นามสกุล", "ชื่อ", "first_name", "firstname":
			if nameCol == -1 {
				nameCol = i
			}
		case "last_name", "lastname", "surname", "นามสกุล", "สกุล":
			if lastNameCol == -1 {
				lastNameCol = i
			}
		}
	}

	if codeCol == -1 {
		codeCol = 0
	}
	if nameCol == -1 {
		if codeCol == 0 {
			nameCol = 1
		} else {
			nameCol = 0
		}
	}
	return codeCol, nameCol, lastNameCol
}

func validateStudentPair(studentCode, fullName string) string {
	if studentCode == "" && fullName == "" {
		return "ไม่พบรหัสนักศึกษาและชื่อ-นามสกุล"
	}
	if studentCode == "" {
		return "ไม่พบรหัสนักศึกษา"
	}
	if len(studentCode) > 30 {
		return "รหัสนักศึกษายาวเกิน 30 ตัวอักษร"
	}
	if strings.HasPrefix(studentCode, "=") || strings.HasPrefix(studentCode, "+") || strings.HasPrefix(studentCode, "@") {
		return "รหัสนักศึกษามีอักขระขึ้นต้นที่ไม่อนุญาต"
	}
	if fullName == "" {
		return "ไม่พบชื่อ-นามสกุล"
	}
	if len(fullName) > 255 {
		return "ชื่อ-นามสกุลยาวเกิน 255 ตัวอักษร"
	}
	if strings.HasPrefix(fullName, "=") || strings.HasPrefix(fullName, "+") || strings.HasPrefix(fullName, "@") {
		return "ชื่อ-นามสกุลมีอักขระขึ้นต้นที่ไม่อนุญาต"
	}
	return ""
}

type BulkStudentItem struct {
	StudentCode string `json:"studentCode"`
	FullName    string `json:"fullName"`
}

// ParseBulkInput parses both structured students slice and raw pasted text.
func ParseBulkInput(items []BulkStudentItem, rawText string) *ParseResult {
	res := &ParseResult{
		ValidRows:   make([]StudentInputRow, 0),
		InvalidRows: make([]InvalidRow, 0),
	}

	rowCounter := 0
	for _, item := range items {
		rowCounter++
		res.TotalRowsRead++
		code := strings.TrimSpace(item.StudentCode)
		name := strings.TrimSpace(item.FullName)
		raw := fmt.Sprintf("%s,%s", code, name)

		if reason := validateStudentPair(code, name); reason != "" {
			res.InvalidRows = append(res.InvalidRows, InvalidRow{
				RowNumber:  rowCounter,
				RawContent: raw,
				Reason:     reason,
			})
			continue
		}

		res.ValidRows = append(res.ValidRows, StudentInputRow{
			RowNumber:   rowCounter,
			StudentCode: code,
			FullName:    name,
			RawContent:  raw,
		})
	}

	if strings.TrimSpace(rawText) != "" {
		lines := strings.Split(strings.ReplaceAll(rawText, "\r\n", "\n"), "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}
			rowCounter++
			res.TotalRowsRead++

			code, name := splitPastedLine(trimmedLine)
			if rowCounter == 1 && isHeaderRow([]string{code, name}) {
				res.TotalRowsRead--
				rowCounter--
				continue
			}

			if reason := validateStudentPair(code, name); reason != "" {
				res.InvalidRows = append(res.InvalidRows, InvalidRow{
					RowNumber:  rowCounter,
					RawContent: trimmedLine,
					Reason:     reason,
				})
				continue
			}

			res.ValidRows = append(res.ValidRows, StudentInputRow{
				RowNumber:   rowCounter,
				StudentCode: code,
				FullName:    name,
				RawContent:  trimmedLine,
			})
		}
	}

	return res
}

func splitPastedLine(line string) (string, string) {
	if strings.Contains(line, "\t") {
		parts := strings.SplitN(line, "\t", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(strings.ReplaceAll(parts[1], "\t", " "))
	}
	if strings.Contains(line, ",") {
		parts := strings.SplitN(line, ",", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(strings.ReplaceAll(parts[1], ",", " "))
	}
	if strings.Contains(line, ";") {
		parts := strings.SplitN(line, ";", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(strings.ReplaceAll(parts[1], ";", " "))
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	return fields[0], strings.Join(fields[1:], " ")
}

// BuildSampleXLSX creates a valid OpenXML .xlsx byte slice from a 2D slice of strings.
func BuildSampleXLSX(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/></sheets>
</workbook>`,
	}

	var sheetXML strings.Builder
	sheetXML.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sheetXML.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)

	for rIdx, row := range rows {
		sheetXML.WriteString(fmt.Sprintf(`<row r="%d">`, rIdx+1))
		for cIdx, val := range row {
			colLetter := string(rune('A' + cIdx))
			var escaped bytes.Buffer
			_ = xml.EscapeText(&escaped, []byte(val))
			sheetXML.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, colLetter, rIdx+1, escaped.String()))
		}
		sheetXML.WriteString(`</row>`)
	}
	sheetXML.WriteString(`</sheetData></worksheet>`)
	files["xl/worksheets/sheet1.xml"] = sheetXML.String()

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

