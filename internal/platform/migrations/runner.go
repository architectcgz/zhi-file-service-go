package migrations

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var migrationFilePattern = regexp.MustCompile(`^([0-9]{6,})_([a-z0-9_]+)\.(up|down)\.sql$`)

// Migration 描述一组版本配对（up/down）的迁移文件。
type Migration struct {
	Version  int64
	Name     string
	Schema   string
	UpPath   string
	DownPath string
	UpFile   string
	DownFile string
}

type migrationPair struct {
	version int64
	name    string
	schema  string
	upPath  string
	downPath string
	upFile  string
	downFile string
}

// Collect 扫描 migrations/<schema>/ 目录并返回按版本号升序排序的 migration 列表。
func Collect(sourceRoot string) ([]Migration, error) {
	dirs, err := os.ReadDir(sourceRoot)
	if err != nil {
		return nil, fmt.Errorf("read source root: %w", err)
	}

	pairsByVersion := make(map[int64]*migrationPair)

	for _, dir := range dirs {
		if !dir.IsDir() || strings.HasPrefix(dir.Name(), ".") {
			continue
		}

		schemaDir := filepath.Join(sourceRoot, dir.Name())
		files, err := os.ReadDir(schemaDir)
		if err != nil {
			return nil, fmt.Errorf("read schema dir %q: %w", schemaDir, err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			name := file.Name()
			match := migrationFilePattern.FindStringSubmatch(name)
			if match == nil {
				continue
			}

			version, err := strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse version %q: %w", match[1], err)
			}

			migrationName := match[2]
			direction := match[3]
			fullPath := filepath.Join(schemaDir, name)

			pair, exists := pairsByVersion[version]
			if !exists {
				pair = &migrationPair{
					version: version,
					name:    migrationName,
					schema:  dir.Name(),
				}
				pairsByVersion[version] = pair
			}

			if pair.name != migrationName {
				return nil, fmt.Errorf(
					"duplicate version %06d has inconsistent names: %q in %q, %q in %q",
					version,
					pair.name,
					pair.schema,
					migrationName,
					dir.Name(),
				)
			}

			if pair.schema != dir.Name() {
				return nil, fmt.Errorf(
					"duplicate version %06d across schemas: %q and %q",
					version,
					pair.schema,
					dir.Name(),
				)
			}

			switch direction {
			case "up":
				if pair.upPath != "" {
					return nil, fmt.Errorf("duplicate up migration for version %06d", version)
				}
				pair.upPath = fullPath
				pair.upFile = name
			case "down":
				if pair.downPath != "" {
					return nil, fmt.Errorf("duplicate down migration for version %06d", version)
				}
				pair.downPath = fullPath
				pair.downFile = name
			default:
				return nil, fmt.Errorf("unsupported direction %q", direction)
			}
		}
	}

	versions := make([]int64, 0, len(pairsByVersion))
	for version := range pairsByVersion {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	result := make([]Migration, 0, len(versions))
	for _, version := range versions {
		pair := pairsByVersion[version]
		if pair.upPath == "" || pair.downPath == "" {
			return nil, fmt.Errorf("version %06d must contain both up and down sql", version)
		}

		result = append(result, Migration{
			Version:  pair.version,
			Name:     pair.name,
			Schema:   pair.schema,
			UpPath:   pair.upPath,
			DownPath: pair.downPath,
			UpFile:   pair.upFile,
			DownFile: pair.downFile,
		})
	}

	return result, nil
}

// BuildLinearView 生成 golang-migrate 可消费的扁平执行目录。
func BuildLinearView(sourceRoot, outputDir string) ([]Migration, error) {
	migrations, err := Collect(sourceRoot)
	if err != nil {
		return nil, err
	}

	if err := ensureOutputDir(outputDir); err != nil {
		return nil, err
	}

	for _, migration := range migrations {
		if err := copyFile(migration.UpPath, filepath.Join(outputDir, migration.UpFile)); err != nil {
			return nil, err
		}
		if err := copyFile(migration.DownPath, filepath.Join(outputDir, migration.DownFile)); err != nil {
			return nil, err
		}
	}

	return migrations, nil
}

func ensureOutputDir(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %q: %w", outputDir, err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("read output dir %q: %w", outputDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		path := filepath.Join(outputDir, entry.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove stale migration %q: %w", path, err)
		}
	}

	return nil
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source %q: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open target %q: %w", targetPath, err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("copy %q -> %q: %w", sourcePath, targetPath, err)
	}

	return nil
}
