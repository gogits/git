package git

import (
	"os"
	"path/filepath"
)

func IsTagExist(repoPath, tagName string) bool {
	tagPath := filepath.Join(repoPath, "refs/tags", tagName)
	return isFile(tagPath)
}

func (repo *Repository) IsTagExist(tagName string) bool {
	return IsTagExist(repo.Path, tagName)
}

// GetTags returns all tags of given repository.
func (repo *Repository) GetTags() ([]string, error) {
	dirPath := filepath.Join(repo.Path, "refs/tags")
	f, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fis, err := f.Readdir(0)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(fis))
	for _, fi := range fis {
		names = append(names, fi.Name())
	}

	return names, nil
}

func CreateTag(repoPath, tagName, id string) error {
	return CreateRef("tags", repoPath, tagName, id)
}

func (repo *Repository) CreateTag(tagName, id string) error {
	return CreateTag(repo.Path, tagName, id)
}
