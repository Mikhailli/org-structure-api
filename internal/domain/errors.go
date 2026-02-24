package domain

import "errors"

// Определение бизнес-ошибок
var (
	ErrDepartmentNotFound      = errors.New("department not found")
	ErrEmployeeNotFound        = errors.New("employee not found")
	ErrDuplicateDepartmentName = errors.New("department with this name already exists in the same parent")
	ErrSelfReference           = errors.New("department cannot be its own parent")
	ErrCyclicReference         = errors.New("moving department would create a cycle")
	ErrInvalidDeleteMode       = errors.New("invalid delete mode")
	ErrReassignTargetRequired  = errors.New("reassign_to_department_id is required when mode is reassign")
	ErrReassignTargetNotFound  = errors.New("target department for reassignment not found")
	ErrCannotReassignToSelf    = errors.New("cannot reassign employees to the same department being deleted")
)
