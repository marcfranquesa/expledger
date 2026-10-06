package report

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func parseCSV(source []byte, xColumn string, yColumns []string) (*Data, error) {
	reader := csv.NewReader(bytes.NewReader(source))
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("CSV header contains an empty column name")
		}
		if _, found := columns[name]; found {
			return nil, fmt.Errorf("CSV header contains duplicate column %q", name)
		}
		columns[name] = i
	}
	for _, name := range append([]string{xColumn}, yColumns...) {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("CSV is missing column %q", name)
		}
	}
	data := &Data{Series: make([]Series, len(yColumns))}
	for i, name := range yColumns {
		data.Series[i] = Series{Name: name}
	}
	hasValues := false
	for row := 1; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", row, err)
		}
		if row > maxCSVRows {
			return nil, fmt.Errorf("CSV exceeds %d data rows", maxCSVRows)
		}
		x, err := numericCell(record[columns[xColumn]], row, xColumn)
		if err != nil {
			return nil, err
		}
		if len(data.X) > 0 && x <= data.X[len(data.X)-1] {
			return nil, fmt.Errorf("CSV row %d: x column %q must be strictly increasing in file order", row, xColumn)
		}
		data.X = append(data.X, x)
		for i, name := range yColumns {
			value := strings.TrimSpace(record[columns[name]])
			var point *float64
			if value != "" {
				number, err := numericCell(value, row, name)
				if err != nil {
					return nil, err
				}
				point = &number
				hasValues = true
			}
			data.Series[i].Values = append(data.Series[i].Values, point)
		}
	}
	if !hasValues {
		return nil, nil
	}
	return data, nil
}

func numericCell(value string, row int, column string) (float64, error) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("CSV row %d: column %q must contain a finite number; got %q", row, column, value)
	}
	return number, nil
}
