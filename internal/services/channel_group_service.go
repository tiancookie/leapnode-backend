package services

import (
	"errors"
	"fmt"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
)

// ChannelGroupService 密钥分组服务
type ChannelGroupService struct {
	db *gorm.DB
}

// NewChannelGroupService 创建密钥分组服务实例
func NewChannelGroupService(db *gorm.DB) *ChannelGroupService {
	return &ChannelGroupService{db: db}
}

// ChannelGroupCreateInput 创建分组请求
type ChannelGroupCreateInput struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Enabled     *bool  `json:"enabled"` // 指针类型，允许显式设置 false
}

// ChannelGroupUpdateInput 更新分组请求
type ChannelGroupUpdateInput struct {
	Name        string `json:"name" binding:"max=100"`
	Description string `json:"description"`
	Priority    *int   `json:"priority"`
	Enabled     *bool  `json:"enabled"`
}

// ChannelGroupRelationInput 分配渠道请求
type ChannelGroupRelationInput struct {
	ChannelIDs []int `json:"channel_ids" binding:"required"` // 允许空数组清空渠道
	Weight     int   `json:"weight"`
}

// ChannelGroupDTO 分组响应 DTO
type ChannelGroupDTO struct {
	ID            uint   `json:"id"`
	DistributorID int    `json:"distributor_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Priority      int    `json:"priority"`
	Enabled       bool   `json:"enabled"`
	ChannelCount  int    `json:"channel_count"` // 关联的渠道数量
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// CreateChannelGroup 创建密钥分组
func (s *ChannelGroupService) CreateChannelGroup(distributorID int, input *ChannelGroupCreateInput) (*models.ChannelGroup, error) {
	group := &models.ChannelGroup{
		DistributorID: distributorID,
		Name:          input.Name,
		Description:   input.Description,
		Priority:      input.Priority,
		Enabled:       true,
	}

	if input.Enabled != nil {
		group.Enabled = *input.Enabled
	}

	if err := s.db.Create(group).Error; err != nil {
		return nil, fmt.Errorf("failed to create key group: %w", err)
	}

	return group, nil
}

// GetChannelGroups 查询分站的密钥分组列表
func (s *ChannelGroupService) GetChannelGroups(distributorID int) ([]*ChannelGroupDTO, error) {
	var groups []models.ChannelGroup
	if err := s.db.Where("distributor_id = ?", distributorID).
		Order("priority DESC, id ASC").
		Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to get key groups: %w", err)
	}

	dtos := make([]*ChannelGroupDTO, len(groups))
	for i, g := range groups {
		// 查询每个分组的渠道数量
		var count int64
		s.db.Model(&models.ChannelGroupRelation{}).Where("group_id = ?", g.ID).Count(&count)

		dtos[i] = &ChannelGroupDTO{
			ID:            g.ID,
			DistributorID: g.DistributorID,
			Name:          g.Name,
			Description:   g.Description,
			Priority:      g.Priority,
			Enabled:       g.Enabled,
			ChannelCount:  int(count),
			CreatedAt:     g.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:     g.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return dtos, nil
}

// GetChannelGroup 根据ID查询单个分组
func (s *ChannelGroupService) GetChannelGroup(distributorID int, groupID uint) (*models.ChannelGroup, error) {
	var group models.ChannelGroup
	if err := s.db.Where("id = ? AND distributor_id = ?", groupID, distributorID).
		First(&group).Error; err != nil {
		return nil, fmt.Errorf("key group not found: %w", err)
	}
	return &group, nil
}

// UpdateChannelGroup 更新密钥分组
func (s *ChannelGroupService) UpdateChannelGroup(distributorID int, groupID uint, input *ChannelGroupUpdateInput) error {
	updates := make(map[string]interface{})

	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	if input.Priority != nil {
		updates["priority"] = *input.Priority
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}

	if len(updates) == 0 {
		return nil // 没有需要更新的字段
	}

	result := s.db.Model(&models.ChannelGroup{}).
		Where("id = ? AND distributor_id = ?", groupID, distributorID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update key group: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("key group not found or no permission")
	}

	return nil
}

// DeleteChannelGroup 删除密钥分组
func (s *ChannelGroupService) DeleteChannelGroup(distributorID int, groupID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 检查分组是否存在且有权限
		var group models.ChannelGroup
		if err := tx.Where("id = ? AND distributor_id = ?", groupID, distributorID).First(&group).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("key group not found or no permission")
			}
			return fmt.Errorf("failed to query key group: %w", err)
		}

		// 2. 检查是否有关联的渠道
		var channelCount int64
		if err := tx.Model(&models.ChannelGroupRelation{}).Where("group_id = ?", groupID).Count(&channelCount).Error; err != nil {
			return fmt.Errorf("failed to count channels: %w", err)
		}

		if channelCount > 0 {
			return fmt.Errorf("cannot delete key group with %d assigned channels, remove channels first", channelCount)
		}

		// 3. 删除分组本身
		result := tx.Where("id = ? AND distributor_id = ?", groupID, distributorID).
			Delete(&models.ChannelGroup{})

		if result.Error != nil {
			return fmt.Errorf("failed to delete key group: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("key group not found or no permission")
		}

		return nil
	})
}

// AssignChannels 为分组分配渠道
func (s *ChannelGroupService) AssignChannels(distributorID int, groupID uint, input *ChannelGroupRelationInput) error {
	// 1. 验证分组存在且属于该分站
	if _, err := s.GetChannelGroup(distributorID, groupID); err != nil {
		return err
	}

	// 2. 验证所有渠道都存在（可选，这里简化处理）
	// TODO: 可以加上验证渠道是否属于该分站

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 3. 删除旧的关联
		if err := tx.Where("group_id = ?", groupID).Delete(&models.ChannelGroupRelation{}).Error; err != nil {
			return fmt.Errorf("failed to clear old channels: %w", err)
		}

		// 4. 批量插入新关联
		weight := input.Weight
		if weight <= 0 {
			weight = 1
		}

		channels := make([]models.ChannelGroupRelation, len(input.ChannelIDs))
		for i, channelID := range input.ChannelIDs {
			channels[i] = models.ChannelGroupRelation{
				GroupID:   groupID,
				ChannelID: channelID,
				Weight:    weight,
			}
		}

		if err := tx.Create(&channels).Error; err != nil {
			return fmt.Errorf("failed to assign channels: %w", err)
		}

		return nil
	})
}

// GetGroupChannels 查询分组关联的渠道列表
func (s *ChannelGroupService) GetGroupChannels(distributorID int, groupID uint) ([]int, error) {
	// 验证分组存在
	if _, err := s.GetChannelGroup(distributorID, groupID); err != nil {
		return nil, err
	}

	var relations []models.ChannelGroupRelation
	if err := s.db.Where("group_id = ?", groupID).Find(&relations).Error; err != nil {
		return nil, fmt.Errorf("failed to get group channels: %w", err)
	}

	channelIDs := make([]int, len(relations))
	for i, r := range relations {
		channelIDs[i] = r.ChannelID
	}

	return channelIDs, nil
}
