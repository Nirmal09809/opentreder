package types

import (
	"github.com/google/uuid"
)

func MustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
