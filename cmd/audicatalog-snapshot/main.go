package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	if len(os.Args) < 2 {
		logger.Fatalf("usage: audicatalog-snapshot <export|import|sync> [flags]")
	}

	switch os.Args[1] {
	case "export":
		if err := runExport(os.Args[2:]); err != nil {
			logger.Fatalf("export failed: %v", err)
		}
	case "import":
		if err := runImport(os.Args[2:]); err != nil {
			logger.Fatalf("import failed: %v", err)
		}
	case "sync":
		if err := runSync(os.Args[2:]); err != nil {
			logger.Fatalf("sync failed: %v", err)
		}
	default:
		logger.Fatalf("unknown command %q", os.Args[1])
	}
}

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	source := fs.String("source", "", "source sqlite db path")
	output := fs.String("output", "", "output snapshot path")
	_ = fs.Parse(args)

	if *source == "" || *output == "" {
		return fmt.Errorf("source and output are required")
	}
	return backupSQLite(*source, *output, false)
}

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	input := fs.String("input", "", "input snapshot path")
	target := fs.String("target", "", "target sqlite db path")
	_ = fs.Parse(args)

	if *input == "" || *target == "" {
		return fmt.Errorf("input and target are required")
	}

	if err := ensureParentDir(*target); err != nil {
		return err
	}
	tempPath := *target + ".importing"
	if err := copyFile(*input, tempPath); err != nil {
		return err
	}
	if err := syncPath(tempPath); err != nil {
		return err
	}
	if err := os.Rename(tempPath, *target); err != nil {
		return fmt.Errorf("rename import: %w", err)
	}
	return syncPath(filepath.Dir(*target))
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	source := fs.String("source", "", "source sqlite db path")
	target := fs.String("target", "", "target sqlite db path")
	_ = fs.Parse(args)

	if *source == "" || *target == "" {
		return fmt.Errorf("source and target are required")
	}
	return backupSQLite(*source, *target, true)
}

func backupSQLite(sourcePath string, targetPath string, createIfMissing bool) error {
	if err := ensureParentDir(targetPath); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sourceDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", sourcePath))
	if err != nil {
		return fmt.Errorf("open source sqlite: %w", err)
	}
	defer sourceDB.Close()

	targetDSN := targetPath
	if createIfMissing {
		targetDSN = fmt.Sprintf("file:%s?_busy_timeout=5000", targetPath)
	}
	targetDB, err := sql.Open("sqlite3", targetDSN)
	if err != nil {
		return fmt.Errorf("open target sqlite: %w", err)
	}
	defer targetDB.Close()

	if err := sourceDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping source sqlite: %w", err)
	}
	if err := targetDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping target sqlite: %w", err)
	}

	targetConn, err := targetDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("target conn: %w", err)
	}
	defer targetConn.Close()

	sourceConn, err := sourceDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("source conn: %w", err)
	}
	defer sourceConn.Close()

	return targetConn.Raw(func(targetDriverConn any) error {
		targetSQLiteConn, ok := targetDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("target connection is not sqlite")
		}

		return sourceConn.Raw(func(sourceDriverConn any) error {
			sourceSQLiteConn, ok := sourceDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("source connection is not sqlite")
			}

			backup, err := targetSQLiteConn.Backup("main", sourceSQLiteConn, "main")
			if err != nil {
				return fmt.Errorf("start backup: %w", err)
			}

			for {
				done, stepErr := backup.Step(128)
				if stepErr != nil {
					_ = backup.Finish()
					return fmt.Errorf("backup step: %w", stepErr)
				}
				if done {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			return backup.Finish()
		})
	})
}

func ensureParentDir(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func copyFile(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open target file: %w", err)
	}
	defer func() {
		_ = target.Close()
	}()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	return target.Sync()
}

func syncPath(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open path for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync path: %w", err)
	}
	return nil
}
