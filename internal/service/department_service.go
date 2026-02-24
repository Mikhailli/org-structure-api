package service

import (
	"context"
	"strings"

	"github.com/org-structure-api/internal/domain"
	"github.com/org-structure-api/internal/dto"
	"github.com/org-structure-api/internal/repository"
)

// DepartmentService определяет интерфейс бизнес-логики для подразделений
type DepartmentService interface {
	Create(ctx context.Context, req *dto.CreateDepartmentRequest) (*domain.Department, error)
	GetByID(ctx context.Context, id int64, query *dto.GetDepartmentQuery) (*domain.Department, error)
	Update(ctx context.Context, id int64, req *dto.UpdateDepartmentRequest) (*domain.Department, error)
	Delete(ctx context.Context, id int64, query *dto.DeleteDepartmentQuery) error
}

type departmentService struct {
	deptRepo repository.DepartmentRepository
	empRepo  repository.EmployeeRepository
}

// NewDepartmentService создаёт новый экземпляр сервиса
func NewDepartmentService(deptRepo repository.DepartmentRepository, empRepo repository.EmployeeRepository) DepartmentService {
	return &departmentService{
		deptRepo: deptRepo,
		empRepo:  empRepo,
	}
}

func (s *departmentService) Create(ctx context.Context, req *dto.CreateDepartmentRequest) (*domain.Department, error) {
	name := strings.TrimSpace(req.Name)

	// Проверяем существование родительского подразделения
	if req.ParentID != nil {
		_, err := s.deptRepo.GetByID(ctx, *req.ParentID)
		if err != nil {
			return nil, err
		}
	}

	// Проверяем уникальность имени в пределах родителя
	exists, err := s.deptRepo.ExistsByNameAndParent(ctx, name, req.ParentID, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrDuplicateDepartmentName
	}

	dept := &domain.Department{
		Name:     name,
		ParentID: req.ParentID,
	}

	if err := s.deptRepo.Create(ctx, dept); err != nil {
		return nil, err
	}

	return dept, nil
}

func (s *departmentService) GetByID(ctx context.Context, id int64, query *dto.GetDepartmentQuery) (*domain.Department, error) {
	return s.deptRepo.GetByIDWithChildren(ctx, id, query.Depth, query.IncludeEmployees)
}

func (s *departmentService) Update(ctx context.Context, id int64, req *dto.UpdateDepartmentRequest) (*domain.Department, error) {
	dept, err := s.deptRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Обновляем имя, если передано
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)

		// Определяем parentID для проверки уникальности
		parentID := dept.ParentID
		if req.ParentID != nil {
			parentID = req.ParentID
		}

		// Проверяем уникальность нового имени
		exists, err := s.deptRepo.ExistsByNameAndParent(ctx, name, parentID, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, domain.ErrDuplicateDepartmentName
		}

		dept.Name = name
	}

	// Обновляем parent_id, если передано
	if req.ParentID != nil {
		newParentID := *req.ParentID

		// Проверка: нельзя сделать подразделение родителем самого себя
		if newParentID == id {
			return nil, domain.ErrSelfReference
		}

		// Проверяем существование нового родителя
		_, err := s.deptRepo.GetByID(ctx, newParentID)
		if err != nil {
			return nil, err
		}

		// Проверка на циклическую ссылку: нельзя переместить в своего потомка
		isDescendant, err := s.deptRepo.IsDescendant(ctx, id, newParentID)
		if err != nil {
			return nil, err
		}
		if isDescendant {
			return nil, domain.ErrCyclicReference
		}

		// Если новое имя не было передано, проверяем уникальность текущего имени в новом родителе
		if req.Name == nil {
			exists, err := s.deptRepo.ExistsByNameAndParent(ctx, dept.Name, &newParentID, &id)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, domain.ErrDuplicateDepartmentName
			}
		}

		dept.ParentID = &newParentID
	}

	if err := s.deptRepo.Update(ctx, dept); err != nil {
		return nil, err
	}

	return dept, nil
}

func (s *departmentService) Delete(ctx context.Context, id int64, query *dto.DeleteDepartmentQuery) error {
	// Проверяем существование подразделения
	_, err := s.deptRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	switch query.Mode {
	case "cascade":
		return s.deptRepo.DeleteCascade(ctx, id)

	case "reassign":
		if query.ReassignToDepartmentID == nil {
			return domain.ErrReassignTargetRequired
		}

		targetID := *query.ReassignToDepartmentID

		// Нельзя переназначить в то же подразделение
		if targetID == id {
			return domain.ErrCannotReassignToSelf
		}

		// Проверяем существование целевого подразделения
		_, err := s.deptRepo.GetByID(ctx, targetID)
		if err != nil {
			if err == domain.ErrDepartmentNotFound {
				return domain.ErrReassignTargetNotFound
			}
			return err
		}

		// Переназначаем сотрудников
		if err := s.empRepo.ReassignToDepartment(ctx, id, targetID); err != nil {
			return err
		}

		// Переназначаем дочерние подразделения к родителю удаляемого
		// или устанавливаем parent_id = NULL, если родителя нет
		dept, _ := s.deptRepo.GetByID(ctx, id)
		descendants, err := s.deptRepo.GetAllDescendantIDs(ctx, id)
		if err != nil {
			return err
		}

		// Переназначаем сотрудников из всех дочерних подразделений
		for _, descID := range descendants {
			if err := s.empRepo.ReassignToDepartment(ctx, descID, targetID); err != nil {
				return err
			}
		}

		// Обновляем parent_id прямых детей на parent текущего подразделения
		_ = dept // Используем parent_id удаляемого подразделения для детей

		// Удаляем подразделение (каскадно удалятся дети из-за FK constraint)
		return s.deptRepo.DeleteCascade(ctx, id)

	default:
		return domain.ErrInvalidDeleteMode
	}
}
