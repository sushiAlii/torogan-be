package models

type Role struct {
	ID   uint   `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"type:varchar(50);unique;not null"`
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

func (Role) TableName() string {
	return "roles"
}
