package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCnyBalanceToUsd(t *testing.T) {
	origRate := operation_setting.USDExchangeRate
	t.Cleanup(func() { operation_setting.USDExchangeRate = origRate })

	operation_setting.USDExchangeRate = 7
	usd, err := cnyBalanceToUsd(70)
	require.NoError(t, err)
	assert.Equal(t, 10.0, usd)

	operation_setting.USDExchangeRate = 7.3
	usd, err = cnyBalanceToUsd(73)
	require.NoError(t, err)
	assert.InDelta(t, 10.0, usd, 1e-9)

	usd, err = cnyBalanceToUsd(0)
	require.NoError(t, err)
	assert.Equal(t, 0.0, usd)

	operation_setting.USDExchangeRate = 0
	_, err = cnyBalanceToUsd(10)
	require.Error(t, err)

	operation_setting.USDExchangeRate = -1
	_, err = cnyBalanceToUsd(10)
	require.Error(t, err)
}
