package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"drip/internal/db"
)

func TestImportFileAndDedupe(t *testing.T) {
	path := realPDFPath(t)

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	ctx := context.Background()

	res := ImportFile(ctx, conn, path)
	if res.Err != nil {
		t.Fatalf("ImportFile: %v", res.Err)
	}
	if res.Source != SourceHDFCPDF || res.Format != "pdf" {
		t.Errorf("source/format = %s/%s", res.Source, res.Format)
	}
	if len(res.Records) != wantTxns {
		t.Fatalf("want %d records, got %d", wantTxns, len(res.Records))
	}
	if res.Imported != wantTxns || res.Duplicates != 0 {
		t.Errorf("first import: imported=%d dup=%d, want %d/0", res.Imported, res.Duplicates, wantTxns)
	}

	cnt, err := db.CountStatements(ctx, conn)
	if err != nil || cnt != wantTxns {
		t.Fatalf("statements in db = %d, %v; want %d", cnt, err, wantTxns)
	}

	audits, err := db.ListImports(ctx, conn, 5)
	if err != nil || len(audits) != 1 {
		t.Fatalf("audits = %d, %v; want 1", len(audits), err)
	}
	if audits[0].Source != string(SourceHDFCPDF) || audits[0].File != "hdfc.pdf" {
		t.Errorf("audit = %+v", audits[0])
	}

	// second import: everything is a duplicate
	res2 := ImportFile(ctx, conn, path)
	if res2.Err != nil {
		t.Fatalf("ImportFile 2: %v", res2.Err)
	}
	if res2.Imported != 0 || res2.Duplicates != wantTxns {
		t.Errorf("second import: imported=%d dup=%d, want 0/%d", res2.Imported, res2.Duplicates, wantTxns)
	}
	cnt, _ = db.CountStatements(ctx, conn)
	if cnt != wantTxns {
		t.Errorf("statements after re-import = %d, want %d", cnt, wantTxns)
	}
}

func TestImportDirSkipsNonStatements(t *testing.T) {
	path := realPDFPath(t)

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	dir := t.TempDir()
	// copy the real pdf under its expected name
	writeFile(t, filepath.Join(dir, "hdfc.pdf"), readFile(t, path))
	// junk files must be ignored
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("not a statement"))
	writeFile(t, filepath.Join(dir, "README.txt"), []byte("readme"))

	results, err := ImportDir(context.Background(), conn, dir)
	if err != nil {
		t.Fatalf("ImportDir: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Err != nil || len(results[0].Records) != wantTxns {
		t.Fatalf("result = %+v", results[0])
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
