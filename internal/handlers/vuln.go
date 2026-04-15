package handlers

import (
	"net/http"
	"skillhub/internal/service"

	"github.com/gin-gonic/gin"
)

// VulnHandler 漏洞数据库状态处理器
type VulnHandler struct {
	vulnMgr *service.MultiVulnDBManager
}

// NewVulnHandler 创建漏洞数据库处理器
func NewVulnHandler(vulnMgr *service.MultiVulnDBManager) *VulnHandler {
	return &VulnHandler{vulnMgr: vulnMgr}
}

// GetStatus 获取漏洞数据库状态
func (h *VulnHandler) GetStatus(c *gin.Context) {
	status := h.vulnMgr.GetStatus()
	c.JSON(http.StatusOK, status)
}
