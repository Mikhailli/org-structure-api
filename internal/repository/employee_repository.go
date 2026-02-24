package repository

import (
	"context"

	"github.com/org-structure-api/internal/domain"
	"gorm.io/gorm"
)

// EmployeeRepository определяет интерфейс для работы с сотрудниками
type EmployeeRepository interface {
	Create(ctx context.Context, emp *domain.Employee) error
	GetByID(ctx context.Context, id int64) (*domain.Employee, error)
	GetByDepartmentID(ctx context.Context, departmentID int64) ([]domain.Employee, error)
	Update(ctx context.Context, emp *domain.Employee) error
	Delete(ctx context.Context, id int64) error
	ReassignToDepartment(ctx context.Context, fromDeptID, toDeptID int64) error
}

type employeeRepository struct {
	db *gorm.DB
}

// NewEmployeeRepository создаёт новый экземпляр репозитория
func NewEmployeeRepository(db *gorm.DB) EmployeeRepository {
	return &employeeRepository{db: db}
}

func (r *employeeRepository) Create(ctx context.Context, emp *domain.Employee) error {
	return r.db.WithContext(ctx).Create(emp).Error
}

func (r *employeeRepository) GetByID(ctx context.Context, id int64) (*domain.Employee, error) {
	var emp domain.Employee
	err := r.db.WithContext(ctx).First(&emp, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrEmployeeNotFound
		}
		return nil, err
	}
	return &emp, nil
}

func (r *employeeRepository) GetByDepartmentID(ctx context.Context, departmentID int64) ([]domain.Employee, error) {
	var employees []domain.Employee
	err := r.db.WithContext(ctx).
		Where("department_id = ?", departmentID).
		Order("created_at ASC").
		Find(&employees).Error
	return employees, err
}

func (r *employeeRepository) Update(ctx context.Context, emp *domain.Employee) error {
	return r.db.WithContext(ctx).Save(emp).Error
}

func (r *employeeRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&domain.Employee{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrEmployeeNotFound
	}
	return nil
}

func (r *employeeRepository) ReassignToDepartment(ctx context.Context, fromDeptID, toDeptID int64) error {
	return r.db.WithContext(ctx).
		Model(&domain.Employee{}).
		Where("department_id = ?", fromDeptID).
		Update("department_id", toDeptID).Error
}
