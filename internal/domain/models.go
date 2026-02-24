package domain

import (
	"time"
)

// Department представляет подразделение организации
type Department struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(200);not null"`
	ParentID  *int64    `json:"parent_id" gorm:"index"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`

	Parent    *Department  `json:"-" gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
	Children  []Department `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Employees []Employee   `json:"employees,omitempty" gorm:"foreignKey:DepartmentID;constraint:OnDelete:CASCADE"`
}

// TableName задаёт имя таблицы для GORM
func (Department) TableName() string {
	return "departments"
}

// Employee представляет сотрудника
type Employee struct {
	ID           int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	DepartmentID int64      `json:"department_id" gorm:"not null;index"`
	FullName     string     `json:"full_name" gorm:"type:varchar(200);not null"`
	Position     string     `json:"position" gorm:"type:varchar(200);not null"`
	HiredAt      *time.Time `json:"hired_at" gorm:"type:date"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`

	Department *Department `json:"-" gorm:"foreignKey:DepartmentID"`
}

// TableName задаёт имя таблицы для GORM
func (Employee) TableName() string {
	return "employees"
}
