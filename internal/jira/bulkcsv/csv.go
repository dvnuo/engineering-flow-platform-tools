package bulkcsv

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strings"
)

func ParseCSV(path string, sampleRows int) (CSVData, error) {
	f, err := os.Open(path)
	if err != nil {
		return CSVData{}, InvalidArgs("failed to read CSV: %s", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	headers, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return CSVData{}, InvalidArgs("CSV is empty")
		}
		return CSVData{}, InvalidArgs("failed to read CSV header: %s", err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(strings.TrimPrefix(headers[i], "\ufeff"))
		if headers[i] == "" {
			return CSVData{}, InvalidArgs("CSV header at column %d is empty", i+1)
		}
	}
	seen := map[string]int{}
	for i, h := range headers {
		if first, ok := seen[h]; ok {
			return CSVData{}, InvalidArgs("duplicate CSV header %q at columns %d and %d", h, first+1, i+1)
		}
		seen[h] = i
	}

	rows := []CSVRow{}
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return CSVData{}, InvalidArgs("failed to read CSV row %d: %s", len(rows)+2, err)
		}
		values := map[string]string{}
		for i, h := range headers {
			if i < len(record) {
				values[h] = record[i]
			} else {
				values[h] = ""
			}
		}
		rows = append(rows, CSVRow{RowNumber: len(rows) + 2, Values: values})
	}

	samples := []map[string]string{}
	for i, row := range rows {
		if sampleRows >= 0 && i >= sampleRows {
			break
		}
		copyRow := map[string]string{}
		for _, h := range headers {
			copyRow[h] = row.Values[h]
		}
		samples = append(samples, copyRow)
	}
	return CSVData{
		Summary: CSVSummary{Path: path, Columns: headers, RowCount: len(rows), SampleRows: samples},
		Rows:    rows,
	}, nil
}
