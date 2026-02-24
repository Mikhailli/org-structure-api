package service

import (
	"context"
	"strings"
	"time"

	"github.com/org-structure-api/internal/domain"
	"github.com/org-structure-api/internal/dto"
	"github.com/org-structure-api/internal/repository"
)

// EmployeeService определяет интерфейс бизнес-логики для сотрудников
type EmployeeService interface {
	Create(ctx context.Context, departmentID int64, req *dto.CreateEmployeeRequest) (*domain.Employee, error)
	GetByID(ctx context.Context, id int64) (*domain.Employee, error)
	GetByDepartmentID(ctx context.Context, departmentID int64) ([]domain.Employee, error)
}

type employeeService struct {
	empRepo  repository.EmployeeRepository
	deptRepo repository.DepartmentRepository
}

// NewEmployeeService создаёт новый экземпляр сервиса
func NewEmployeeService(empRepo repository.EmployeeRepository, deptRepo repository.DepartmentRepository) EmployeeService {
	return &employeeService{
		empRepo:  empRepo,
		deptRepo: deptRepo,
	}
}

func (s *employeeService) Create(ctx context.Context, departmentID int64, req *dto.CreateEmployeeRequest) (*domain.Employee, error) {
	// Проверяем существование подразделения
	_, err := s.deptRepo.GetByID(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	
	emp := &domain.Employee{
		DepartmentID: departmentID,
		FullName:     strings.TrimSpace(req.FullName),
		Position:     strings.TrimSpace(req.Position),
	}
	
	// Парсим дату найма, если передана
	if req.HiredAt != nil {
		hiredAt, err := time.Parse("2006-01-02", *req.HiredAt)
		if err != nil {
			return nil, err
		}
		emp.HiredAt = &hiredAt
	}
	
	if err := s.empRepo.Create(ctx, emp); err != nil {
		return nil, err
	}
	
	return emp, nil
}

func (s *employeeService) GetByID(ctx context.Context, id int64) (*domain.Employee, error) {
	return s.empRepo.GetByID(ctx, id)
}

func (s *employeeService) GetByDepartmentID(ctx context.Context, departmentID int64) ([]domain.Employee, error) {
	// Проверяем существование подразделения
	_, err := s.deptRepo.GetByID(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	
	return s.empRepo.GetByDepartmentID(ctx, departmentID)
}
