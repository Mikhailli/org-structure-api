package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/org-structure-api/internal/domain"
	"github.com/org-structure-api/internal/dto"
	"github.com/org-structure-api/internal/handler"
)

type mockDepartmentRepo struct {
	departments map[int64]*domain.Department
	nextID      int64
}

func newMockDepartmentRepo() *mockDepartmentRepo {
	return &mockDepartmentRepo{
		departments: make(map[int64]*domain.Department),
		nextID:      1,
	}
}

func (m *mockDepartmentRepo) Create(ctx context.Context, dept *domain.Department) error {
	dept.ID = m.nextID
	dept.CreatedAt = time.Now()
	m.nextID++
	m.departments[dept.ID] = dept
	return nil
}

func (m *mockDepartmentRepo) GetByID(ctx context.Context, id int64) (*domain.Department, error) {
	if dept, ok := m.departments[id]; ok {
		return dept, nil
	}
	return nil, domain.ErrDepartmentNotFound
}

func (m *mockDepartmentRepo) GetByIDWithChildren(ctx context.Context, id int64, depth int, includeEmployees bool) (*domain.Department, error) {
	return m.GetByID(ctx, id)
}

func (m *mockDepartmentRepo) Update(ctx context.Context, dept *domain.Department) error {
	m.departments[dept.ID] = dept
	return nil
}

func (m *mockDepartmentRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.departments[id]; !ok {
		return domain.ErrDepartmentNotFound
	}
	delete(m.departments, id)
	return nil
}

func (m *mockDepartmentRepo) DeleteCascade(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}

func (m *mockDepartmentRepo) ExistsByNameAndParent(ctx context.Context, name string, parentID *int64, excludeID *int64) (bool, error) {
	for _, dept := range m.departments {
		if dept.Name == name {
			sameParent := (parentID == nil && dept.ParentID == nil) ||
				(parentID != nil && dept.ParentID != nil && *parentID == *dept.ParentID)
			if sameParent {
				if excludeID == nil || dept.ID != *excludeID {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (m *mockDepartmentRepo) IsDescendant(ctx context.Context, ancestorID, descendantID int64) (bool, error) {
	current := descendantID
	visited := make(map[int64]bool)
	for {
		if current == ancestorID {
			return true, nil
		}
		if visited[current] {
			return false, nil
		}
		visited[current] = true
		dept, ok := m.departments[current]
		if !ok || dept.ParentID == nil {
			return false, nil
		}
		current = *dept.ParentID
	}
}

func (m *mockDepartmentRepo) GetAllDescendantIDs(ctx context.Context, id int64) ([]int64, error) {
	var result []int64
	for _, dept := range m.departments {
		if dept.ParentID != nil && *dept.ParentID == id {
			result = append(result, dept.ID)
		}
	}
	return result, nil
}

type mockEmployeeRepo struct {
	employees map[int64]*domain.Employee
	nextID    int64
}

func newMockEmployeeRepo() *mockEmployeeRepo {
	return &mockEmployeeRepo{
		employees: make(map[int64]*domain.Employee),
		nextID:    1,
	}
}

func (m *mockEmployeeRepo) Create(ctx context.Context, emp *domain.Employee) error {
	emp.ID = m.nextID
	emp.CreatedAt = time.Now()
	m.nextID++
	m.employees[emp.ID] = emp
	return nil
}

func (m *mockEmployeeRepo) GetByID(ctx context.Context, id int64) (*domain.Employee, error) {
	if emp, ok := m.employees[id]; ok {
		return emp, nil
	}
	return nil, domain.ErrEmployeeNotFound
}

func (m *mockEmployeeRepo) GetByDepartmentID(ctx context.Context, departmentID int64) ([]domain.Employee, error) {
	var result []domain.Employee
	for _, emp := range m.employees {
		if emp.DepartmentID == departmentID {
			result = append(result, *emp)
		}
	}
	return result, nil
}

func (m *mockEmployeeRepo) Update(ctx context.Context, emp *domain.Employee) error {
	m.employees[emp.ID] = emp
	return nil
}

func (m *mockEmployeeRepo) Delete(ctx context.Context, id int64) error {
	delete(m.employees, id)
	return nil
}

func (m *mockEmployeeRepo) ReassignToDepartment(ctx context.Context, fromDeptID, toDeptID int64) error {
	for _, emp := range m.employees {
		if emp.DepartmentID == fromDeptID {
			emp.DepartmentID = toDeptID
		}
	}
	return nil
}

type mockDepartmentService struct {
	deptRepo *mockDepartmentRepo
	empRepo  *mockEmployeeRepo
}

func (s *mockDepartmentService) Create(ctx context.Context, req *dto.CreateDepartmentRequest) (*domain.Department, error) {
	if req.ParentID != nil {
		if _, err := s.deptRepo.GetByID(ctx, *req.ParentID); err != nil {
			return nil, err
		}
	}

	exists, _ := s.deptRepo.ExistsByNameAndParent(ctx, req.Name, req.ParentID, nil)
	if exists {
		return nil, domain.ErrDuplicateDepartmentName
	}

	dept := &domain.Department{
		Name:     req.Name,
		ParentID: req.ParentID,
	}
	s.deptRepo.Create(ctx, dept)
	return dept, nil
}

func (s *mockDepartmentService) GetByID(ctx context.Context, id int64, query *dto.GetDepartmentQuery) (*domain.Department, error) {
	return s.deptRepo.GetByID(ctx, id)
}

func (s *mockDepartmentService) Update(ctx context.Context, id int64, req *dto.UpdateDepartmentRequest) (*domain.Department, error) {
	dept, err := s.deptRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.ParentID != nil {
		newParentID := *req.ParentID

		if newParentID == id {
			return nil, domain.ErrSelfReference
		}

		if _, err := s.deptRepo.GetByID(ctx, newParentID); err != nil {
			return nil, err
		}

		isDesc, _ := s.deptRepo.IsDescendant(ctx, id, newParentID)
		if isDesc {
			return nil, domain.ErrCyclicReference
		}

		dept.ParentID = req.ParentID
	}

	if req.Name != nil {
		parentID := dept.ParentID
		if req.ParentID != nil {
			parentID = req.ParentID
		}
		exists, _ := s.deptRepo.ExistsByNameAndParent(ctx, *req.Name, parentID, &id)
		if exists {
			return nil, domain.ErrDuplicateDepartmentName
		}
		dept.Name = *req.Name
	}

	s.deptRepo.Update(ctx, dept)
	return dept, nil
}

func (s *mockDepartmentService) Delete(ctx context.Context, id int64, query *dto.DeleteDepartmentQuery) error {
	if _, err := s.deptRepo.GetByID(ctx, id); err != nil {
		return err
	}

	if query.Mode != "cascade" && query.Mode != "reassign" {
		return domain.ErrInvalidDeleteMode
	}

	if query.Mode == "reassign" {
		if query.ReassignToDepartmentID == nil {
			return domain.ErrReassignTargetRequired
		}

		targetID := *query.ReassignToDepartmentID

		if targetID == id {
			return domain.ErrCannotReassignToSelf
		}

		if _, err := s.deptRepo.GetByID(ctx, targetID); err != nil {
			return domain.ErrReassignTargetNotFound
		}

		s.empRepo.ReassignToDepartment(ctx, id, targetID)
	}

	return s.deptRepo.Delete(ctx, id)
}

type mockEmployeeService struct {
	empRepo  *mockEmployeeRepo
	deptRepo *mockDepartmentRepo
}

func (s *mockEmployeeService) Create(ctx context.Context, departmentID int64, req *dto.CreateEmployeeRequest) (*domain.Employee, error) {
	if _, err := s.deptRepo.GetByID(ctx, departmentID); err != nil {
		return nil, err
	}

	emp := &domain.Employee{
		DepartmentID: departmentID,
		FullName:     req.FullName,
		Position:     req.Position,
	}

	if req.HiredAt != nil {
		hiredAt, err := time.Parse("2006-01-02", *req.HiredAt)
		if err != nil {
			return nil, err
		}
		emp.HiredAt = &hiredAt
	}

	s.empRepo.Create(ctx, emp)
	return emp, nil
}

func (s *mockEmployeeService) GetByID(ctx context.Context, id int64) (*domain.Employee, error) {
	return s.empRepo.GetByID(ctx, id)
}

func (s *mockEmployeeService) GetByDepartmentID(ctx context.Context, departmentID int64) ([]domain.Employee, error) {
	return s.empRepo.GetByDepartmentID(ctx, departmentID)
}

type testServer struct {
	server   *httptest.Server
	deptRepo *mockDepartmentRepo
	empRepo  *mockEmployeeRepo
}

func setupTestServer(_ *testing.T) *testServer {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	deptRepo := newMockDepartmentRepo()
	empRepo := newMockEmployeeRepo()

	deptService := &mockDepartmentService{deptRepo: deptRepo, empRepo: empRepo}
	empService := &mockEmployeeService{empRepo: empRepo, deptRepo: deptRepo}

	deptHandler := handler.NewDepartmentHandler(deptService, empService, logger)
	router := handler.NewRouter(deptHandler, logger)

	return &testServer{
		server:   httptest.NewServer(router.Setup()),
		deptRepo: deptRepo,
		empRepo:  empRepo,
	}
}

func (ts *testServer) Close() {
	ts.server.Close()
}

func postJSON(url string, body map[string]any) (*http.Response, error) {
	data, _ := json.Marshal(body)
	return http.Post(url, "application/json", bytes.NewBuffer(data))
}

func patchJSON(url string, body map[string]any) (*http.Response, error) {
	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func deleteRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func mustPost(t *testing.T, url string, body map[string]any) {
	resp, err := postJSON(url, body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
}

func TestHealthCheck(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.server.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestCreateDepartment_Success(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{"name": "IT Department"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var result dto.DepartmentResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Name != "IT Department" {
		t.Errorf("expected name 'IT Department', got '%s'", result.Name)
	}
}

func TestCreateDepartment_WithParent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Parent"})

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{"name": "Child", "parent_id": 1})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

func TestCreateDepartment_EmptyName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{"name": ""})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateDepartment_MissingName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateDepartment_ParentNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{"name": "Child", "parent_id": 999})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestCreateDepartment_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{"name": "IT"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestCreateDepartment_SameNameDifferentParent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Parent1"})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Parent2"})

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Child", "parent_id": 1})

	resp, err := postJSON(ts.server.URL+"/departments/", map[string]any{"name": "Child", "parent_id": 2})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

func TestCreateDepartment_InvalidJSON(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.server.URL+"/departments/", "application/json", bytes.NewBuffer([]byte("invalid")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestGetDepartment_Success(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := http.Get(ts.server.URL + "/departments/1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestGetDepartment_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.server.URL + "/departments/999")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestGetDepartment_InvalidID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.server.URL + "/departments/abc")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateDepartment_Success(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Old Name"})

	resp, err := patchJSON(ts.server.URL+"/departments/1", map[string]any{"name": "New Name"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result dto.DepartmentResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Name != "New Name" {
		t.Errorf("expected 'New Name', got '%s'", result.Name)
	}
}

func TestUpdateDepartment_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := patchJSON(ts.server.URL+"/departments/999", map[string]any{"name": "Test"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestUpdateDepartment_SelfReference(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept"})

	resp, err := patchJSON(ts.server.URL+"/departments/1", map[string]any{"parent_id": 1})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateDepartment_CyclicReference(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Parent"})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Child", "parent_id": 1})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "GrandChild", "parent_id": 2})

	resp, err := patchJSON(ts.server.URL+"/departments/1", map[string]any{"parent_id": 3})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestUpdateDepartment_ParentNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept"})

	resp, err := patchJSON(ts.server.URL+"/departments/1", map[string]any{"parent_id": 999})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestUpdateDepartment_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept1"})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept2"})

	resp, err := patchJSON(ts.server.URL+"/departments/2", map[string]any{"name": "Dept1"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestUpdateDepartment_MoveToAnotherParent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Parent1"})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Parent2"})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Child", "parent_id": 1})

	resp, err := patchJSON(ts.server.URL+"/departments/3", map[string]any{"parent_id": 2})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestDeleteDepartment_Cascade(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "ToDelete"})

	resp, err := deleteRequest(ts.server.URL + "/departments/1?mode=cascade")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
}

func TestDeleteDepartment_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := deleteRequest(ts.server.URL + "/departments/999?mode=cascade")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestDeleteDepartment_InvalidMode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept"})

	resp, err := deleteRequest(ts.server.URL + "/departments/1?mode=invalid")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestDeleteDepartment_ReassignWithoutTarget(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept"})

	resp, err := deleteRequest(ts.server.URL + "/departments/1?mode=reassign")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestDeleteDepartment_ReassignToSelf(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept"})

	resp, err := deleteRequest(ts.server.URL + "/departments/1?mode=reassign&reassign_to_department_id=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestDeleteDepartment_ReassignTargetNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Dept"})

	resp, err := deleteRequest(ts.server.URL + "/departments/1?mode=reassign&reassign_to_department_id=999")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestDeleteDepartment_ReassignSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "Target"})
	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "ToDelete"})

	mustPost(t, ts.server.URL+"/departments/2/employees/", map[string]any{"full_name": "John", "position": "Dev"})

	resp, err := deleteRequest(ts.server.URL + "/departments/2?mode=reassign&reassign_to_department_id=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
}

func TestCreateEmployee_Success(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := postJSON(ts.server.URL+"/departments/1/employees/", map[string]any{
		"full_name": "John Doe",
		"position":  "Developer",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

func TestCreateEmployee_WithHiredAt(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := postJSON(ts.server.URL+"/departments/1/employees/", map[string]any{
		"full_name": "John Doe",
		"position":  "Developer",
		"hired_at":  "2024-01-15",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

func TestCreateEmployee_DepartmentNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := postJSON(ts.server.URL+"/departments/999/employees/", map[string]any{
		"full_name": "John Doe",
		"position":  "Developer",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestCreateEmployee_EmptyFullName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := postJSON(ts.server.URL+"/departments/1/employees/", map[string]any{
		"full_name": "",
		"position":  "Developer",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateEmployee_EmptyPosition(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := postJSON(ts.server.URL+"/departments/1/employees/", map[string]any{
		"full_name": "John Doe",
		"position":  "",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateEmployee_MissingFields(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	resp, err := postJSON(ts.server.URL+"/departments/1/employees/", map[string]any{})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateEmployee_InvalidDepartmentID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := postJSON(ts.server.URL+"/departments/abc/employees/", map[string]any{
		"full_name": "John",
		"position":  "Dev",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	mustPost(t, ts.server.URL+"/departments/", map[string]any{"name": "IT"})

	req, err := http.NewRequest(http.MethodPut, ts.server.URL+"/departments/1", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

func TestFullWorkflow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := postJSON(ts.server.URL+"/departments/", map[string]any{"name": "Company"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create root department")
	}
	resp.Body.Close()

	resp, _ = postJSON(ts.server.URL+"/departments/", map[string]any{"name": "IT", "parent_id": 1})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create IT department")
	}
	resp.Body.Close()

	resp, _ = postJSON(ts.server.URL+"/departments/", map[string]any{"name": "HR", "parent_id": 1})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create HR department")
	}
	resp.Body.Close()

	resp, _ = postJSON(ts.server.URL+"/departments/2/employees/", map[string]any{
		"full_name": "John Developer",
		"position":  "Senior Developer",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create employee")
	}
	resp.Body.Close()

	resp, _ = http.Get(ts.server.URL + "/departments/1?depth=2&include_employees=true")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get department tree")
	}
	resp.Body.Close()

	resp, _ = patchJSON(ts.server.URL+"/departments/2", map[string]any{"name": "IT Department"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to update department")
	}
	resp.Body.Close()

	resp, _ = patchJSON(ts.server.URL+"/departments/3", map[string]any{"parent_id": 2})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to move department")
	}
	resp.Body.Close()

	resp, _ = deleteRequest(ts.server.URL + "/departments/3?mode=cascade")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("failed to delete department")
	}
	resp.Body.Close()

	t.Log("Full workflow completed successfully")
}

func BenchmarkCreateDepartment(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	deptRepo := newMockDepartmentRepo()
	empRepo := newMockEmployeeRepo()
	deptService := &mockDepartmentService{deptRepo: deptRepo, empRepo: empRepo}
	empService := &mockEmployeeService{empRepo: empRepo, deptRepo: deptRepo}
	deptHandler := handler.NewDepartmentHandler(deptService, empService, logger)
	router := handler.NewRouter(deptHandler, logger)
	server := httptest.NewServer(router.Setup())
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body, _ := json.Marshal(map[string]any{"name": "Dept" + strconv.Itoa(i)})
		resp, _ := http.Post(server.URL+"/departments/", "application/json", bytes.NewBuffer(body))
		resp.Body.Close()
	}
}
