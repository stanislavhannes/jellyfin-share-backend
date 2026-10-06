// Package download issues and checks the tickets that authorise a file download.
//
// A download is charged when it is requested, not when bytes move: the browser
// fetches the file in a plain GET it may repeat (a resumed download is a Range
// request), and counting every one of those would charge a viewer several plays
// for one file. So the POST that checks the share's limits hands back a signed,
// short-lived ticket, and the GET that streams the file only has to verify it.
package download

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AllEpisodes as a ticket's item asks for every episode of the share in one ZIP.
const AllEpisodes = "*"

// ticketLifetime bounds how long a ticket can start or resume a download. A
// transfer already running is not cut off when it passes. A ticket is charged
// once but can be fetched again until it expires, so this is kept short: long
// enough to resume an interrupted download, too short to pass around as a
// free copy of the file.
const ticketLifetime = 2 * time.Hour

var ErrInvalidTicket = errors.New("invalid or expired download ticket")

type Ticket struct {
	ShareID uuid.UUID `json:"s"`
	// ItemID is the Jellyfin item to send, or AllEpisodes. It was checked against
	// the share when the ticket was issued; the GET must not take it from anywhere
	// else.
	ItemID    string `json:"i"`
	ExpiresAt int64  `json:"e"`
}

type Signer struct {
	key []byte
}

// NewSigner derives the ticket key from the backend secret, so a ticket signature
// can never double as anything else signed with that secret.
func NewSigner(secret string) *Signer {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("jfshare-download-ticket-v1"))
	return &Signer{key: mac.Sum(nil)}
}

func (s *Signer) Issue(shareID uuid.UUID, itemID string) (string, error) {
	payload, err := json.Marshal(Ticket{
		ShareID:   shareID,
		ItemID:    itemID,
		ExpiresAt: time.Now().Add(ticketLifetime).Unix(),
	})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + base64.RawURLEncoding.EncodeToString(s.sign(body)), nil
}

func (s *Signer) Verify(raw string) (*Ticket, error) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return nil, ErrInvalidTicket
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.sign(body)) {
		return nil, ErrInvalidTicket
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, ErrInvalidTicket
	}
	var t Ticket
	if err := json.Unmarshal(payload, &t); err != nil {
		return nil, ErrInvalidTicket
	}
	if time.Now().Unix() > t.ExpiresAt {
		return nil, ErrInvalidTicket
	}
	return &t, nil
}

func (s *Signer) sign(body string) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(body))
	return mac.Sum(nil)
}
