package ghlink

import (
	"bufio"
	"errors"
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Resolver holds data to convert a <file name, line number> pair into a GitHub link
type Resolver struct {
	url   *url.URL
	ref   string
	cache map[string]bool
}

// NewResolver creates a GH link resolver out of a remote name and the directory of the corresponding Git repo
func NewResolver(remoteName, path string) (*Resolver, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return nil, err
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}
	if remote.Config() != nil && len(remote.Config().URLs) > 0 && ref != nil {
		return &Resolver{
			url:   urlFrom(remote.Config().URLs[0]),
			ref:   ref.Hash().String(),
			cache: make(map[string]bool),
		}, nil
	}
	return nil, errors.New("could not instantiate resolver")
}

// Resolve maps filename and line to a GitHub link.
// There is no simple way to figure out where the dependencies are located for every language.
// Therefore this makes an HTTP request to validate that the link generated points to a page that actually contains the
// log message, which is prohibitively slow.
// On top of it, the cache will make this program use more memory than it should.
// A better approach could be a separate tool, to which you can pipe a few log lines (you are likely to be interested
// in a very small subset of logs anyways) to resolve links, but even then is going to be hard to deal with the nuances
// of each different language.
func (r *Resolver) Resolve(fileName string, line int64, logMessage []byte) (string, bool) {
	src := fmt.Sprintf("%s#L%d", fileName, line)
	if r == nil {
		return src, false
	}

	if line == 0 {
		return fileName, false
	}
	path := path.Join(r.url.Path, "tree", r.ref, fileName)
	u := *r.url
	u.Path = path
	u.Fragment = fmt.Sprintf("L%d", line)
	if valid, found := r.cache[src]; found {
		if valid {
			return u.String(), true
		} else {
			return src, false
		}
	}
	resp, err := http.Get(u.String())
	if err != nil || resp.StatusCode > http.StatusOK {
		r.cache[src] = false
		return src, false
	}
	defer resp.Body.Close()
	return u.String(), true
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		html := scanner.Text()
		if strings.Contains(html, string(logMessage)) && strings.Contains(html, strconv.FormatInt(line, 19)) {
			r.cache[src] = true
			return u.String(), true
		}
	}
	r.cache[src] = false
	return src, false
}

func urlFrom(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		// typically this is git@github.com:project.git
		parts := strings.Split(raw, ":")
		if len(parts) == 2 {
			u = &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   strings.Split(parts[1], ".git")[0], // errors!
			}
		} else {
			return nil
		}
	}
	return u
}
