package cmd

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"

	"github.com/rs/zerolog/log"
)

type importFileInput struct {
	Path  string
	Label string
}

func defaultRemoteFileSource() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "client"
	}
	return hostname
}

func normalizeRemoteFileSource(source string) (string, error) {
	var normalized strings.Builder
	lastSeparator := false
	for _, r := range strings.ToLower(strings.TrimSpace(source)) {
		allowed := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-'
		if allowed {
			normalized.WriteRune(r)
			lastSeparator = false
			continue
		}
		if normalized.Len() > 0 && !lastSeparator {
			normalized.WriteByte('-')
			lastSeparator = true
		}
	}
	value := strings.Trim(normalized.String(), ".-")
	if value == "" {
		return "", fmt.Errorf("--source must contain at least one ASCII letter or digit")
	}
	return value, nil
}

func remoteFileURL(source, filename string) (string, error) {
	normalizedSource, err := normalizeRemoteFileSource(source)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}
	urlPath := filepath.ToSlash(absPath)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	return (&url.URL{Scheme: "remote-file", Host: normalizedSource, Path: urlPath}).String(), nil
}

func expandImportInputs(args []string, directories []*config.Directory) ([]importFileInput, error) {
	inputs := make([]importFileInput, 0)
	seen := make(map[string]bool)
	appendFile := func(path, label string) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if !seen[absPath] {
			seen[absPath] = true
			inputs = append(inputs, importFileInput{Path: absPath, Label: label})
		}
		return nil
	}
	walkDirectory := func(root string, directory *config.Directory) error {
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				log.Warn().Err(walkErr).Str("path", path).Msg("Error accessing path")
				return nil
			}
			if entry.IsDir() {
				if path != root && files.ShouldSkipDir(entry.Name(), directory.Excludes, directory.IncludeHidden) {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := os.Stat(path)
			if err != nil {
				log.Warn().Err(err).Str("path", path).Msg("Failed to inspect path")
				return nil
			}
			if !info.Mode().IsRegular() || !directory.IsMatching(entry.Name()) {
				return nil
			}
			return appendFile(path, directory.Label)
		})
	}

	if len(args) == 0 {
		if len(directories) == 0 {
			return nil, fmt.Errorf("no watched directories are configured")
		}
		for _, directory := range directories {
			if directory == nil {
				continue
			}
			root, err := filepath.Abs(files.ExpandHome(directory.Path))
			if err != nil {
				return nil, fmt.Errorf("resolve watched directory %s: %w", directory.Path, err)
			}
			if err := walkDirectory(root, directory); err != nil {
				return nil, fmt.Errorf("walk watched directory %s: %w", root, err)
			}
		}
	} else {
		for _, input := range args {
			absPath, err := filepath.Abs(files.ExpandHome(input))
			if err != nil {
				return nil, fmt.Errorf("resolve input %s: %w", input, err)
			}
			info, err := os.Stat(absPath)
			if err != nil {
				return nil, fmt.Errorf("inspect input %s: %w", input, err)
			}
			if !info.IsDir() {
				if !info.Mode().IsRegular() {
					return nil, fmt.Errorf("input is not a regular file: %s", input)
				}
				label := ""
				if directory := files.FindMatchingDir(directories, absPath); files.DirectoryMatchesPath(directory, absPath) {
					label = directory.Label
				}
				if err := appendFile(absPath, label); err != nil {
					return nil, err
				}
				continue
			}

			directory := files.FindMatchingDir(directories, absPath)
			if directory == nil {
				directory = &config.Directory{Path: absPath}
			}
			if err := walkDirectory(absPath, directory); err != nil {
				return nil, fmt.Errorf("walk input directory %s: %w", absPath, err)
			}
		}
	}

	if len(args) == 0 {
		sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	}
	return inputs, nil
}

func importRemoteFile(
	c *client.Client,
	input importFileInput,
	content []byte,
	info os.FileInfo,
	source string,
	maxFileSize int64,
	skip bool,
	labelOverride documentLabelOverride,
) (imported, skipped, errCount int) {
	if info.Size() == 0 {
		log.Warn().Str("file", input.Path).Msg("Empty file, skipping")
		return 0, 0, 1
	}
	if maxFileSize > 0 && info.Size() > maxFileSize {
		log.Warn().Int64("size", info.Size()).Int64("limit", maxFileSize).Str("file", input.Path).Msg("File exceeds configured size limit")
		return 0, 0, 1
	}

	remoteURL, err := remoteFileURL(source, input.Path)
	if err != nil {
		log.Warn().Err(err).Str("file", input.Path).Msg("Failed to create remote file URL")
		return 0, 0, 1
	}
	if skip {
		exists, err := c.DocumentExists(remoteURL)
		if err != nil {
			log.Warn().Err(err).Str("url", remoteURL).Msg("Failed to check if remote file exists")
			return 0, 0, 1
		}
		if exists {
			log.Debug().Str("url", remoteURL).Msg("Remote file already exists, skipping")
			return 0, 1, 0
		}
	}

	fallbackLabel := input.Label
	if fallbackLabel == "" {
		fallbackLabel = "import"
	}
	d := &document.Document{
		URL:     remoteURL,
		Updated: info.ModTime().Unix(),
		Type:    document.RemoteFile,
		Label:   labelOverride.resolve("", fallbackLabel),
	}
	if err := indexer.PrepareFileContent(input.Path, d, content); err != nil {
		log.Warn().Err(err).Str("file", input.Path).Msg("Failed to extract file content")
		return 0, 0, 1
	}
	if d.Text == "" && d.HTML == "" {
		log.Warn().Str("file", input.Path).Msg("File contains no indexable content")
		return 0, 0, 1
	}
	if err := c.AddDocumentJSON(d); err != nil {
		log.Warn().Err(err).Str("file", input.Path).Str("url", remoteURL).Msg("Failed to import file snapshot")
		return 0, 0, 1
	}
	return 1, 0, 0
}

func importRemoteFilePath(
	c *client.Client,
	input importFileInput,
	source string,
	maxFileSize int64,
	skip bool,
	labelOverride documentLabelOverride,
) (imported, skipped, errCount int) {
	info, err := os.Stat(input.Path)
	if err != nil {
		log.Warn().Err(err).Str("file", input.Path).Msg("Failed to inspect file")
		return 0, 0, 1
	}
	content, err := os.ReadFile(input.Path)
	if err != nil {
		log.Warn().Err(err).Str("file", input.Path).Msg("Failed to read file")
		return 0, 0, 1
	}
	return importRemoteFile(c, input, content, info, source, maxFileSize, skip, labelOverride)
}
