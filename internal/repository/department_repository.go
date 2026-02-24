package repository

import (
	"context"

	"github.com/org-structure-api/internal/domain"
	"gorm.io/gorm"
)

// DepartmentRepository определяет интерфейс для работы с подразделениями
type DepartmentRepository interface {
	Create(ctx context.Context, dept *domain.Department) error
	GetByID(ctx context.Context, id int64) (*domain.Department, error)
	GetByIDWithChildren(ctx context.Context, id int64, depth int, includeEmployees bool) (*domain.Department, error)
	Update(ctx context.Context, dept *domain.Department) error
	Delete(ctx context.Context, id int64) error
	DeleteCascade(ctx context.Context, id int64) error
	ExistsByNameAndParent(ctx context.Context, name string, parentID *int64, excludeID *int64) (bool, error)
	IsDescendant(ctx context.Context, ancestorID, descendantID int64) (bool, error)
	GetAllDescendantIDs(ctx context.Context, id int64) ([]int64, error)
}

type departmentRepository struct {
	db *gorm.DB
}

// NewDepartmentRepository создаёт новый экземпляр репозитория
func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRepository{db: db}
}

func (r *departmentRepository) Create(ctx context.Context, dept *domain.Department) error {
	return r.db.WithContext(ctx).Create(dept).Error
}

func (r *departmentRepository) GetByID(ctx context.Context, id int64) (*domain.Department, error) {
	var dept domain.Department
	err := r.db.WithContext(ctx).First(&dept, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrDepartmentNotFound
		}
		return nil, err
	}
	return &dept, nil
}

func (r *departmentRepository) GetByIDWithChildren(ctx context.Context, id int64, depth int, includeEmployees bool) (*domain.Department, error) {
	var dept domain.Department

	query := r.db.WithContext(ctx)

	if includeEmployees {
		query = query.Preload("Employees", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		})
	}

	err := query.First(&dept, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrDepartmentNotFound
		}
		return nil, err
	}

	// Рекурсивно загружаем дочерние подразделения
	if depth > 0 {
		if err := r.loadChildren(ctx, &dept, depth, includeEmployees); err != nil {
			return nil, err
		}
	}

	return &dept, nil
}

func (r *departmentRepository) loadChildren(ctx context.Context, dept *domain.Department, depth int, includeEmployees bool) error {
	if depth <= 0 {
		return nil
	}

	query := r.db.WithContext(ctx).Where("parent_id = ?", dept.ID)

	if includeEmployees {
		query = query.Preload("Employees", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		})
	}

	var children []domain.Department
	if err := query.Find(&children).Error; err != nil {
		return err
	}

	for i := range children {
		if err := r.loadChildren(ctx, &children[i], depth-1, includeEmployees); err != nil {
			return err
		}
	}

	dept.Children = children
	return nil
}

func (r *departmentRepository) Update(ctx context.Context, dept *domain.Department) error {
	return r.db.WithContext(ctx).Save(dept).Error
}

func (r *departmentRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&domain.Department{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrDepartmentNotFound
	}
	return nil
}

func (r *departmentRepository) DeleteCascade(ctx context.Context, id int64) error {
	return r.Delete(ctx, id)
}

func (r *departmentRepository) ExistsByNameAndParent(ctx context.Context, name string, parentID *int64, excludeID *int64) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&domain.Department{}).Where("name = ?", name)

	if parentID != nil {
		query = query.Where("parent_id = ?", *parentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	err := query.Count(&count).Error
	return count > 0, err
}

func (r *departmentRepository) IsDescendant(ctx context.Context, ancestorID, descendantID int64) (bool, error) {
	// Рекурсивно проверяем, является ли descendantID потомком ancestorID
	descendants, err := r.GetAllDescendantIDs(ctx, ancestorID)
	if err != nil {
		return false, err
	}

	for _, id := range descendants {
		if id == descendantID {
			return true, nil
		}
	}
	return false, nil
}

func (r *departmentRepository) GetAllDescendantIDs(ctx context.Context, id int64) ([]int64, error) {
	var result []int64

	// Используем рекурсивный CTE для PostgreSQL
	query := `
		WITH RECURSIVE descendants AS (
			SELECT id FROM departments WHERE parent_id = $1
			UNION ALL
			SELECT d.id FROM departments d
			INNER JOIN descendants ds ON d.parent_id = ds.id
		)
		SELECT id FROM descendants
	`

	rows, err := r.db.WithContext(ctx).Raw(query, id).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var descendantID int64
		if err := rows.Scan(&descendantID); err != nil {
			return nil, err
		}
		result = append(result, descendantID)
	}

	return result, rows.Err()
}
