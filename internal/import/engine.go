package importer

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AarnoStormborn/drip/internal/db"
	"github.com/AarnoStormborn/drip/internal/model"
)

// Source identifies the origin of a statement file.
type Source string

const (
	SourceHDFCPDF      Source = "hdfc_pdf"
	SourceHDFCCSV      Source = "hdfc_csv"
	SourceICICIPDF     Source = "icici_pdf"
	SourceICICICSV     Source = "icici_csv"
	SourceGPAYPDF      Source = "gpay_pdf"
	SourceGPAYMandates Source = "gpay_mandates"
)

// Result is the outcome of importing one file.
type Result struct {
	File       string
	Source     Source
	Format     string // csv|pdf|xls|text
	Records    []model.StatementRecord
	Imported   int64 // rows inserted (0 if all were duplicates)
	Duplicates int64
	Err        error
}

// DetectSource identifies a statement file from its filename/extension.
// Returns an error for unknown or not-yet-supported files.
func DetectSource(path string) (Source, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		// TODO: distinguish ICICI / GPay PDFs by content sniffing.
		return SourceHDFCPDF, "pdf", nil
	case ".csv":
		return SourceHDFCCSV, "csv", nil
	case ".xlsx", ".xls":
		return SourceHDFCCSV, ext[1:], nil
	default:
		return "", "", fmt.Errorf("unsupported file type %q (supported: .pdf, .csv, .xlsx)", ext)
	}
}

// ImportFile parses one statement file and inserts the records into the
// database (deduplicated against existing rows), recording an imports audit.
func ImportFile(ctx context.Context, q *sql.DB, path string) Result {
	res := Result{File: filepath.Base(path)}
	src, format, err := DetectSource(path)
	res.Source, res.Format = src, format
	if err != nil {
		res.Err = err
		return res
	}

	var recs []model.StatementRecord
	switch src {
	case SourceHDFCPDF:
		recs, err = ParseHDFCPDF(path)
	default:
		err = fmt.Errorf("parser for %s (%s) is not implemented yet — supported: HDFC bank statement PDF", src, format)
	}
	if err != nil {
		res.Err = err
		return res
	}
	res.Records = recs

	// Dedupe against existing rows (same date + description + amount).
	existing, err := db.AllStatementKeys(ctx, q)
	if err != nil {
		res.Err = err
		return res
	}
	fresh := recs[:0]
	for _, r := range recs {
		if existing[statementKey(r)] {
			res.Duplicates++
			continue
		}
		fresh = append(fresh, r)
	}
	if len(fresh) > 0 {
		n, err := db.InsertStatements(ctx, q, fresh)
		if err != nil {
			res.Err = err
			return res
		}
		res.Imported = n
	}

	audit := model.ImportAudit{Source: string(src), Format: format, File: res.File, Matched: len(recs)}
	if _, err := db.InsertImport(ctx, q, &audit); err != nil {
		res.Err = err
		return res
	}
	return res
}

func statementKey(r model.StatementRecord) string {
	return r.Date + "|" + r.Description + "|" + fmt.Sprint(r.Amount)
}

// ImportDir imports every supported statement file in dir (non-recursive).
// Returns results sorted by filename, one per candidate file.
func ImportDir(ctx context.Context, q *sql.DB, dir string) ([]Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".pdf" || ext == ".csv" || ext == ".xlsx" || ext == ".xls" {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)

	results := make([]Result, 0, len(paths))
	for _, p := range paths {
		results = append(results, ImportFile(ctx, q, p))
	}
	return results, nil
}
