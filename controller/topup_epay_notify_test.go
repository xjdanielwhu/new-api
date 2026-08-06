package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWriteEpayNotifyResponseSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/user/epay/notify", nil)

	writeEpayNotifyResponse(c, true)

	assert.Equal(t, "success", recorder.Body.String())
}

// 额度入账失败时必须回 fail，支付平台才会重试；提前 ACK 会导致收款成功但额度永久丢失。
func TestWriteEpayNotifyResponseFailureAsksForRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/user/epay/notify", nil)

	writeEpayNotifyResponse(c, false)

	assert.Equal(t, "fail", recorder.Body.String())
}
