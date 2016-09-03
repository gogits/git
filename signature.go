package git

import (
	"bytes"
	"strconv"
	"time"
)

// Author and Committer information
type Signature struct {
	Email string
	Name  string
	When  time.Time
}

// Outputs the author signature in the same format as git log:
// "Author <email>". If needed, the "When" must be manually added to the
// string.
func (s Signature) String() string {
	return s.Name + " <" + s.Email + ">"
}

// Helper to get a signature from the commit line, which looks like this:
//     author Patrick Gundlach <gundlach@speedata.de> 1378823654 +0200
// but without the "author " at the beginning (this method should)
// be used for author and committer.
//
// FIXME: include timezone!
func newSignatureFromCommitline(line []byte) (*Signature, error) {
	sig := new(Signature)
	emailstart := bytes.IndexByte(line, '<')
	sig.Name = string(line[:emailstart-1])
	emailstop := bytes.IndexByte(line, '>')
	sig.Email = string(line[emailstart+1 : emailstop])
	timestop := bytes.IndexByte(line[emailstop+2:], ' ')
	timestring := string(line[emailstop+2 : emailstop+2+timestop])
	seconds, err := strconv.ParseInt(timestring, 10, 64)
	if err != nil {
		return nil, err
	}
	sig.When = time.Unix(seconds, 0)
	return sig, nil
}
