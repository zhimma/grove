package ulid

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

func New() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	return id.String()
}