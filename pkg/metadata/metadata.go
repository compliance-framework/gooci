package metadata

import (
	"encoding/json"
	"os"
	"path"
	"time"
)

type Data struct {
	Artifacts *[]Artifact
	Metadata  *Metadata
}

type Artifact struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Goos    string `json:"goos"`
	Goarch  string `json:"goarch"`
	Goarm64 string `json:"goarm64"`
	Kind    string `json:"type"`
}

type Metadata struct {
	ProjectName string    `json:"project_name"`
	Description string    `json:"description"`
	Tag         string    `json:"tag"`
	Version     string    `json:"version"`
	Commit      string    `json:"commit"`
	Date        time.Time `json:"date"`
}

func New(dir string) (Data, error) {
	// dir is assumed to be an absolute path

	metadataJson, err := os.ReadFile(path.Join(dir, "metadata.json"))
	if err != nil {
		return Data{}, err
	}

	metadata := &Metadata{}

	err = json.Unmarshal(metadataJson, metadata)
	if err != nil {
		return Data{}, err
	}

	artifactJson, err := os.ReadFile(path.Join(dir, "artifacts.json"))
	if err != nil {
		return Data{}, err
	}

	artifacts := &[]Artifact{}

	err = json.Unmarshal(artifactJson, artifacts)
	if err != nil {
		return Data{}, err
	}

	return Data{
		Artifacts: artifacts,
		Metadata:  metadata,
	}, nil
}

func (d *Data) GetArchives() *[]Artifact {
	var archives []Artifact
	for _, artifact := range *d.Artifacts {
		if artifact.Kind == "Archive" {
			archives = append(archives, artifact)
		}
	}
	return &archives
}
