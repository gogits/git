package git

import (
	"container/list"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Unknwon/com"
)

var (
	refRexp = regexp.MustCompile("ref: (.*)\n")
)

// get branch's last commit or a special commit by id string
func (repo *Repository) GetCommitOfBranch(branchName string) (*Commit, error) {
	commitId, err := repo.GetCommitIdOfBranch(branchName)
	if err != nil {
		return nil, err
	}

	return repo.GetCommit(commitId)
}

func (repo *Repository) GetCommitIdOfBranch(branchName string) (string, error) {
	return repo.GetCommitIdOfRef("refs/heads/" + branchName)
}

func (repo *Repository) GetCommitOfTag(tagName string) (*Commit, error) {
	commitId, err := repo.GetCommitIdOfTag(tagName)
	if err != nil {
		return nil, err
	}

	return repo.GetCommit(commitId)
}

func (repo *Repository) GetCommitIdOfTag(tagName string) (string, error) {
	return repo.GetCommitIdOfRef("refs/tags/" + tagName)
}

func (repo *Repository) GetCommitIdOfRef(refpath string) (string, error) {
start:
	f, err := ioutil.ReadFile(filepath.Join(repo.Path, refpath))
	if err != nil {
		return "", err
	}

	allMatches := refRexp.FindAllStringSubmatch(string(f), 1)
	if allMatches == nil {
		// let's assume this is a sha1
		sha1 := string(f[:40])
		if !IsSha1(sha1) {
			return "", fmt.Errorf("heads file wrong sha1 string %s", sha1)
		}
		return sha1, nil
	}
	// yes, it's "ref: something". Now let's lookup "something"
	refpath = allMatches[0][1]
	goto start
}

// Find the commit object in the repository.
func (repo *Repository) GetCommit(commitId string) (*Commit, error) {
	id, err := NewIdFromString(commitId)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}

func (repo *Repository) getCommit(id sha1) (*Commit, error) {
	if repo.commitCache != nil {
		if c, ok := repo.commitCache[id]; ok {
			return c, nil
		}
	} else {
		repo.commitCache = make(map[sha1]*Commit, 10)
	}

	_, _, dataRc, err := repo.getRawObject(id, false)
	if err != nil {
		return nil, err
	}

	defer func() {
		dataRc.Close()
	}()

	// TODO reader
	data, err := ioutil.ReadAll(dataRc)
	if err != nil {
		return nil, err
	}

	commit, err := parseCommitData(data)
	if err != nil {
		return nil, err
	}
	commit.repo = repo
	commit.Id = id

	repo.commitCache[id] = commit

	return commit, nil
}

func (repo *Repository) CommitsCount(commitId string) (int, error) {
	id, err := NewIdFromString(commitId)
	if err != nil {
		return 0, err
	}
	return repo.commitsCount(id)
}

func (repo *Repository) FileCommitsCount(branch, file string) (int, error) {
	stdout, stderr, err := com.ExecCmdDir(repo.Path, "git", "rev-list", "--count",
		branch, "--", file)
	if err != nil {
		return 0, errors.New(stderr)
	}
	return StrToInt(strings.TrimSpace(stdout))
}

func (repo *Repository) commitsCount(id sha1) (int, error) {
	stdout, stderr, err := com.ExecCmdDir(repo.Path, "git", "rev-list", "--count", id.String())
	if err != nil {
		return 0, errors.New(stderr)
	}
	return StrToInt(strings.TrimSpace(stdout))
}

// used only for single tree, (]
func (repo *Repository) CommitsBetween(last *Commit, before *Commit) (*list.List, error) {
	l := list.New()
	if last == nil || last.ParentCount() == 0 {
		return l, nil
	}

	var err error
	cur := last
	for {
		if cur.Id.Equal(before.Id) {
			break
		}
		l.PushBack(cur)
		if cur.ParentCount() == 0 {
			break
		}
		cur, err = cur.Parent(0)
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

func (repo *Repository) CommitsBefore(commitId string) (*list.List, error) {
	id, err := NewIdFromString(commitId)
	if err != nil {
		return nil, err
	}

	return repo.getCommitsBefore(id)
}

func (repo *Repository) getCommitsBefore(id sha1) (*list.List, error) {
	l := list.New()
	lock := new(sync.Mutex)
	err := repo.commitsBefore(lock, l, nil, id, 0)
	return l, err
}

func (repo *Repository) commitsBefore(lock *sync.Mutex, l *list.List, parent *list.Element, id sha1, limit int) error {
	commit, err := repo.getCommit(id)
	if err != nil {
		return err
	}

	var e *list.Element
	if parent == nil {
		e = l.PushBack(commit)
	} else {
		var in = parent
		//lock.Lock()
		for {
			if in == nil {
				break
			} else if in.Value.(*Commit).Id.Equal(commit.Id) {
				//lock.Unlock()
				return nil
			} else {
				if in.Next() == nil {
					break
				}
				if in.Value.(*Commit).Committer.When.Equal(commit.Committer.When) {
					break
				}

				if in.Value.(*Commit).Committer.When.After(commit.Committer.When) &&
					in.Next().Value.(*Commit).Committer.When.Before(commit.Committer.When) {
					break
				}
			}
			in = in.Next()
		}

		e = l.InsertAfter(commit, in)
		//lock.Unlock()
	}

	var pr = parent
	if commit.ParentCount() > 1 {
		pr = e
	}

	for i := 0; i < commit.ParentCount(); i++ {
		id, err := commit.ParentId(i)
		if err != nil {
			return err
		}
		err = repo.commitsBefore(lock, l, pr, id, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

// SearchCommits searches commits in given commitId and keyword of repository.
func (repo *Repository) SearchCommits(commitId, keyword string) (*list.List, error) {
	id, err := NewIdFromString(commitId)
	if err != nil {
		return nil, err
	}

	return repo.searchCommits(id, keyword)
}
func (repo *Repository) searchCommits(id sha1, keyword string) (*list.List, error) {
	stdout, stderr, err := com.ExecCmdDirBytes(repo.Path, "git", "log", id.String(), "-100",
		"-i", "--grep="+keyword, prettyLogFormat)
	if err != nil {
		return nil, err
	} else if len(stderr) > 0 {
		return nil, errors.New(string(stderr))
	}
	return parsePrettyFormatLog(repo, stdout)
}

// GetCommitsByRange returns certain number of commits with given page of repository.
func (repo *Repository) CommitsByRange(commitId string, page int) (*list.List, error) {
	id, err := NewIdFromString(commitId)
	if err != nil {
		return nil, err
	}

	return repo.commitsByRange(id, page)
}

func (repo *Repository) commitsByRange(id sha1, page int) (*list.List, error) {
	stdout, stderr, err := com.ExecCmdDirBytes(repo.Path, "git", "log", id.String(),
		"--skip="+IntToStr((page-1)*50), "--max-count=50", prettyLogFormat)
	if err != nil {
		return nil, errors.New(string(stderr))
	}
	return parsePrettyFormatLog(repo, stdout)
}

func (repo *Repository) CommitsByFileAndRange(branch, file string, page int) (*list.List, error) {
	stdout, stderr, err := com.ExecCmdDirBytes(repo.Path, "git", "log", branch,
		"--skip="+IntToStr((page-1)*50), "--max-count=50", prettyLogFormat, "--", file)
	if err != nil {
		return nil, errors.New(string(stderr))
	}
	return parsePrettyFormatLog(repo, stdout)
}

func (repo *Repository) GetCommitOfRelPath(commitId, relPath string) (*Commit, error) {
	id, err := NewIdFromString(commitId)
	if err != nil {
		return nil, err
	}

	return repo.getCommitOfRelPath(id, relPath)
}

func (repo *Repository) getCommitOfRelPath(id sha1, relPath string) (*Commit, error) {
	stdout, _, err := com.ExecCmdDir(repo.Path, "git", "log", "-1", prettyLogFormat, id.String(), "--", relPath)
	if err != nil {
		return nil, err
	}

	id, err = NewIdFromString(string(stdout))
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}
