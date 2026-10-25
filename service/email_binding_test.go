package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAccountEmailEnforcesAliasAndDomainRestrictions(t *testing.T) {
	previousDomainRestriction := common.EmailDomainRestrictionEnabled
	previousAliasRestriction := common.EmailAliasRestrictionEnabled
	previousWhitelist := common.EmailDomainWhitelist
	t.Cleanup(func() {
		common.EmailDomainRestrictionEnabled = previousDomainRestriction
		common.EmailAliasRestrictionEnabled = previousAliasRestriction
		common.EmailDomainWhitelist = previousWhitelist
	})

	common.EmailDomainRestrictionEnabled = true
	common.EmailAliasRestrictionEnabled = true
	common.EmailDomainWhitelist = []string{"gmail.com"}

	email, err := ValidateAccountEmail("Person@GMAIL.COM")
	require.NoError(t, err)
	assert.Equal(t, "person@gmail.com", email)

	for _, email := range []string{
		"person+bot@gmail.com",
		"p.erson@gmail.com",
		"person@maildrop.cc",
	} {
		_, err := ValidateAccountEmail(email)
		require.ErrorIs(t, err, ErrAccountEmailRestricted, email)
	}
}
