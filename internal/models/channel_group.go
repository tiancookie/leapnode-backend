package models

import "time"

// ChannelGroup 渠道分组模型（原名 KeyGroup，避免与定价分组冲突）
//
// 用途: 分站可创建多个渠道分组（如"高优先级"、"备用池"），每个分组关联多个 channels。
// 用户请求时可根据规则（如负载均衡、优先级）路由到不同分组。
type ChannelGroup struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	DistributorID  int       `gorm:"index;not null" json:"distributor_id"`       // 所属分站
	Name           string    `gorm:"type:varchar(100);not null" json:"name"`     // 分组名称
	Description    string    `gorm:"type:text" json:"description"`               // 分组描述
	Priority       int       `gorm:"default:0" json:"priority"`                  // 优先级（数字越大优先级越高）
	Enabled        bool      `gorm:"default:true" json:"enabled"`                // 是否启用
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName 指定表名
func (ChannelGroup) TableName() string {
	return "channel_groups"
}

// ChannelGroupRelation 渠道分组与渠道的关联表
type ChannelGroupRelation struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	GroupID    uint      `gorm:"index;not null" json:"group_id"`    // 分组ID
	ChannelID  int       `gorm:"index;not null" json:"channel_id"`  // 渠道ID
	Weight     int       `gorm:"default:1" json:"weight"`           // 权重（用于负载均衡）
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 指定表名
func (ChannelGroupRelation) TableName() string {
	return "channel_group_relations"
}
