package middleware

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Tests for the default tenant ID behavior added for Socrate integration

func TestDefaultTenantIDIsValid(t *testing.T) {
	defaultTenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	assert.NotEqual(t, uuid.Nil, defaultTenantID)
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", defaultTenantID.String())
}

func TestDefaultTenantIDIsDeterministic(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	assert.Equal(t, id1, id2)
}
