package service

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 钱包结算必须遵守“余额不为负”：余额不足时扣完剩余额度，差额记为冲销并进入
// 消费日志的 admin_info，避免结算无条件扣减把余额扣成负数。
func TestWalletSettlementClampsAtZeroBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	model.DB = db
	// 直写数据库，避免批量更新与缓存的异步行为影响断言。
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))

	for index, tc := range []struct {
		name string
		// held 是预扣费之后、结算之前的余额。
		held          int
		preConsumed   int
		actualQuota   int
		wantBalance   int
		wantOverdraft int
	}{
		{
			name:        "sufficient balance settles the full delta",
			held:        9000,
			preConsumed: 1000,
			actualQuota: 4000,
			wantBalance: 6000,
		},
		{
			name:          "insufficient balance clamps at zero and writes off the rest",
			held:          200,
			preConsumed:   800,
			actualQuota:   5000,
			wantBalance:   0,
			wantOverdraft: 4000,
		},
		{
			name:        "actual charge below the reservation refunds without overdraft",
			held:        0,
			preConsumed: 500,
			actualQuota: 100,
			wantBalance: 400,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := model.User{Username: fmt.Sprintf("wallet_overdraft_%d", index), Quota: tc.held, Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&user).Error)
			token := model.Token{UserId: user.Id, Key: fmt.Sprintf("wallet-overdraft-%d", index), Name: "wallet-overdraft", RemainQuota: 10_000_000, Status: common.TokenStatusEnabled}
			require.NoError(t, db.Create(&token).Error)
			t.Cleanup(func() {
				require.NoError(t, db.Unscoped().Delete(&token).Error)
				require.NoError(t, db.Unscoped().Delete(&user).Error)
			})

			info := &relaycommon.RelayInfo{UserId: user.Id, TokenId: token.Id, TokenKey: token.Key}
			session := &BillingSession{
				relayInfo:        info,
				funding:          &WalletFunding{userId: user.Id},
				preConsumedQuota: tc.preConsumed,
				tokenConsumed:    tc.preConsumed,
			}
			require.NoError(t, session.Settle(tc.actualQuota))

			balance, err := model.GetUserQuota(user.Id, false)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBalance, balance, "余额不允许被扣成负数")
			assert.Equal(t, tc.wantOverdraft, info.WalletOverdraftQuota)

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			other := model.NewLogOther()
			attachWalletOverdraft(ctx, info, other)
			adminInfo, _ := other.Snapshot()["admin_info"].(map[string]any)
			if tc.wantOverdraft == 0 {
				assert.Nil(t, adminInfo["wallet_overdraft"])
				return
			}
			assert.Equal(t, tc.wantOverdraft, adminInfo["wallet_overdraft"])
		})
	}
}
