// SPDX-License-Identifier: AGPL-3.0-or-later

package indexer

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/blevesearch/bleve/v2"
)

const (
	indexVersionKey    = "hister.index_version"
	analyzerConfigKey  = "hister.analyzer_fingerprint"
	embeddingConfigKey = "hister.embedding_fingerprint"
)

type indexMetadata struct {
	Version              int
	AnalyzerFingerprint  string
	EmbeddingFingerprint string
}

func configuredIndexMetadata(detectLanguages, keepStopwords bool, embeddingFingerprint string) indexMetadata {
	return indexMetadata{
		Version:              Version,
		AnalyzerFingerprint:  AnalyzerFingerprint(detectLanguages, keepStopwords),
		EmbeddingFingerprint: embeddingFingerprint,
	}
}

func readIndexMetadata(idx bleve.Index) (indexMetadata, bool, error) {
	metadata := indexMetadata{Version: -1}

	version, err := idx.GetInternal([]byte(indexVersionKey))
	if err != nil {
		return metadata, false, fmt.Errorf("read index version: %w", err)
	}
	if len(version) > 0 {
		metadata.Version, err = strconv.Atoi(string(version))
		if err != nil {
			return metadata, false, fmt.Errorf("parse index version %q: %w", version, err)
		}
	}

	fingerprint, err := idx.GetInternal([]byte(analyzerConfigKey))
	if err != nil {
		return metadata, false, fmt.Errorf("read analyzer configuration fingerprint: %w", err)
	}
	metadata.AnalyzerFingerprint = string(fingerprint)

	embeddingFingerprint, err := idx.GetInternal([]byte(embeddingConfigKey))
	if err != nil {
		return metadata, false, fmt.Errorf("read embedding configuration fingerprint: %w", err)
	}
	metadata.EmbeddingFingerprint = string(embeddingFingerprint)

	complete := metadata.Version >= 0 && metadata.AnalyzerFingerprint != ""
	return metadata, complete, nil
}

func writeIndexMetadata(idx bleve.Index, metadata indexMetadata) error {
	if metadata.Version < 0 {
		return errors.New("index version is missing")
	}
	if metadata.AnalyzerFingerprint == "" {
		return errors.New("analyzer configuration fingerprint is missing")
	}
	if err := idx.SetInternal(
		[]byte(analyzerConfigKey),
		[]byte(metadata.AnalyzerFingerprint),
	); err != nil {
		return fmt.Errorf("store analyzer configuration fingerprint: %w", err)
	}
	if metadata.EmbeddingFingerprint == "" {
		if err := idx.DeleteInternal([]byte(embeddingConfigKey)); err != nil {
			return fmt.Errorf("clear embedding configuration fingerprint: %w", err)
		}
	} else if err := idx.SetInternal(
		[]byte(embeddingConfigKey),
		[]byte(metadata.EmbeddingFingerprint),
	); err != nil {
		return fmt.Errorf("store embedding configuration fingerprint: %w", err)
	}
	if err := idx.SetInternal(
		[]byte(indexVersionKey),
		[]byte(strconv.Itoa(metadata.Version)),
	); err != nil {
		return fmt.Errorf("store index version: %w", err)
	}
	return nil
}

func (i *Indexer) GetMetadata() (int, string, error) {
	metadata, err := i.getMetadata()
	return metadata.Version, metadata.AnalyzerFingerprint, err
}

// GetEmbeddingFingerprint returns the fingerprint of the configuration used
// to build the stored document embeddings. An empty value means it is unknown.
func (i *Indexer) GetEmbeddingFingerprint() (string, error) {
	metadata, err := i.getMetadata()
	return metadata.EmbeddingFingerprint, err
}

func (i *Indexer) backfillEmbeddingFingerprint(fingerprint string) error {
	if fingerprint == "" {
		return nil
	}
	metadata, err := i.getMetadata()
	if err != nil {
		return err
	}
	if metadata.EmbeddingFingerprint != "" {
		return nil
	}
	metadata.EmbeddingFingerprint = fingerprint
	return i.setMetadata(metadata)
}

func (i *Indexer) SetMetadata(version int, analyzerFingerprint string) error {
	metadata, err := i.getMetadata()
	if err != nil {
		return err
	}
	metadata.Version = version
	metadata.AnalyzerFingerprint = analyzerFingerprint
	return i.setMetadata(metadata)
}

func (i *Indexer) getMetadata() (indexMetadata, error) {
	i.indexesMu.RLock()
	defer i.indexesMu.RUnlock()
	metadata := indexMetadata{Version: -1}
	versionsComplete := true
	fingerprintsComplete := true
	fingerprintIndex := ""
	embeddingFingerprintsComplete := true
	embeddingFingerprintIndex := ""

	for name, idx := range i.indexers {
		subIndexMetadata, _, err := readIndexMetadata(idx)
		if err != nil {
			return metadata, fmt.Errorf("read metadata from %s: %w", name, err)
		}

		if subIndexMetadata.Version < 0 {
			versionsComplete = false
		} else if metadata.Version < 0 || subIndexMetadata.Version < metadata.Version {
			metadata.Version = subIndexMetadata.Version
		}

		if subIndexMetadata.AnalyzerFingerprint == "" {
			fingerprintsComplete = false
		} else if metadata.AnalyzerFingerprint == "" {
			metadata.AnalyzerFingerprint = subIndexMetadata.AnalyzerFingerprint
			fingerprintIndex = name
		} else if metadata.AnalyzerFingerprint != subIndexMetadata.AnalyzerFingerprint {
			return indexMetadata{Version: -1}, fmt.Errorf(
				"analyzer configuration fingerprints differ between %s and %s",
				fingerprintIndex,
				name,
			)
		}

		if subIndexMetadata.EmbeddingFingerprint == "" {
			embeddingFingerprintsComplete = false
		} else if metadata.EmbeddingFingerprint == "" {
			metadata.EmbeddingFingerprint = subIndexMetadata.EmbeddingFingerprint
			embeddingFingerprintIndex = name
		} else if metadata.EmbeddingFingerprint != subIndexMetadata.EmbeddingFingerprint {
			return indexMetadata{Version: -1}, fmt.Errorf(
				"embedding configuration fingerprints differ between %s and %s",
				embeddingFingerprintIndex,
				name,
			)
		}
	}

	if !versionsComplete {
		metadata.Version = -1
	}
	if !fingerprintsComplete {
		metadata.AnalyzerFingerprint = ""
	}
	if !embeddingFingerprintsComplete {
		metadata.EmbeddingFingerprint = ""
	}
	return metadata, nil
}

func (i *Indexer) setMetadata(metadata indexMetadata) error {
	i.indexesMu.Lock()
	defer i.indexesMu.Unlock()
	for name, idx := range i.indexers {
		if err := writeIndexMetadata(idx, metadata); err != nil {
			return fmt.Errorf("store metadata in %s: %w", name, err)
		}
	}
	return nil
}

func copyIndexMetadata(source, target bleve.Index) error {
	metadata, complete, err := readIndexMetadata(source)
	if err != nil {
		return fmt.Errorf("read source index metadata: %w", err)
	}
	if !complete {
		return errors.New("source index metadata is incomplete")
	}
	if err := writeIndexMetadata(target, metadata); err != nil {
		return fmt.Errorf("store target index metadata: %w", err)
	}
	return nil
}
