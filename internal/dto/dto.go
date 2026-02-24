package dto

import (
	"time"
)

// CreateDepartmentRequest - запрос на создание подразделения
type CreateDepartmentRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=200"`
	ParentID *int64 `json:"parent_id" validate:"omitempty,min=1"`
}

// UpdateDepartmentRequest - запрос на обновление подразделения
type UpdateDepartmentRequest struct {
	Name     *string `json:"name" validate:"omitempty,min=1,max=200"`
	ParentID *int64  `json:"parent_id" validate:"omitempty,min=1"`
}

// CreateEmployeeRequest - запрос на создание сотрудника
type CreateEmployeeRequest struct {
	FullName string  `json:"full_name" validate:"required,min=1,max=200"`
	Position string  `json:"position" validate:"required,min=1,max=200"`
	HiredAt  *string `json:"hired_at" validate:"omitempty,datetime=2006-01-02"`
}

// DepartmentResponse - ответ с данными подразделения
type DepartmentResponse struct {
	ID        int64                 `json:"id"`
	Name      string                `json:"name"`
	ParentID  *int64                `json:"parent_id"`
	CreatedAt time.Time             `json:"created_at"`
	Employees []EmployeeResponse    `json:"employees,omitempty"`
	Children  []DepartmentResponse  `json:"children,omitempty"`
}

// EmployeeResponse - ответ с данными сотрудника
type EmployeeResponse struct {
	ID           int64      `json:"id"`
	DepartmentID int64      `json:"department_id"`
	FullName     string     `json:"full_name"`
	Position     string     `json:"position"`
	HiredAt      *string    `json:"hired_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ErrorResponse - стандартный ответ с ошибкой
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// DeleteDepartmentQuery - параметры запроса удаления
type DeleteDepartmentQuery struct {
	Mode                   string `validate:"required,oneof=cascade reassign"`
	ReassignToDepartmentID *int64 `validate:"required_if=Mode reassign,omitempty,min=1"`
}

// GetDepartmentQuery - параметры запроса получения подразделения
type GetDepartmentQuery struct {
	Depth            int  `validate:"min=1,max=5"`
	IncludeEmployees bool
}
