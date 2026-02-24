package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/org-structure-api/internal/domain"
	"github.com/org-structure-api/internal/dto"
	"github.com/org-structure-api/internal/service"
)

type DepartmentHandler struct {
	deptService service.DepartmentService
	empService  service.EmployeeService
	validator   *validator.Validate
	logger      *slog.Logger
}

func NewDepartmentHandler(
	deptService service.DepartmentService,
	empService service.EmployeeService,
	logger *slog.Logger,
) *DepartmentHandler {
	return &DepartmentHandler{
		deptService: deptService,
		empService:  empService,
		validator:   validator.New(),
		logger:      logger,
	}
}

func (h *DepartmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDepartmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation error", err.Error())
		return
	}

	dept, err := h.deptService.Create(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, h.toDepartmentResponse(dept))
}

func (h *DepartmentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid department id", err.Error())
		return
	}

	query := h.parseGetQuery(r)
	if err := h.validator.Struct(&query); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation error", err.Error())
		return
	}

	dept, err := h.deptService.GetByID(r.Context(), id, &query)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, h.toDepartmentResponseWithChildren(dept, query.IncludeEmployees))
}

func (h *DepartmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid department id", err.Error())
		return
	}

	var req dto.UpdateDepartmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation error", err.Error())
		return
	}

	dept, err := h.deptService.Update(r.Context(), id, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, h.toDepartmentResponse(dept))
}

func (h *DepartmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid department id", err.Error())
		return
	}

	query := h.parseDeleteQuery(r)
	if err := h.validator.Struct(&query); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation error", err.Error())
		return
	}

	if err := h.deptService.Delete(r.Context(), id, &query); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DepartmentHandler) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	deptID, err := h.extractID(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid department id", err.Error())
		return
	}

	var req dto.CreateEmployeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation error", err.Error())
		return
	}

	emp, err := h.empService.Create(r.Context(), deptID, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, h.toEmployeeResponse(emp))
}

func (h *DepartmentHandler) extractID(r *http.Request) (int64, error) {
	path := strings.TrimPrefix(r.URL.Path, "/departments/")
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, "/employees")

	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, errors.New("id is required")
	}

	return strconv.ParseInt(parts[0], 10, 64)
}

func (h *DepartmentHandler) parseGetQuery(r *http.Request) dto.GetDepartmentQuery {
	query := dto.GetDepartmentQuery{
		Depth:            1,
		IncludeEmployees: true,
	}

	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if depth, err := strconv.Atoi(depthStr); err == nil {
			query.Depth = depth
		}
	}

	if includeStr := r.URL.Query().Get("include_employees"); includeStr != "" {
		query.IncludeEmployees = includeStr == "true"
	}

	return query
}

func (h *DepartmentHandler) parseDeleteQuery(r *http.Request) dto.DeleteDepartmentQuery {
	query := dto.DeleteDepartmentQuery{
		Mode: r.URL.Query().Get("mode"),
	}

	if reassignStr := r.URL.Query().Get("reassign_to_department_id"); reassignStr != "" {
		if reassignID, err := strconv.ParseInt(reassignStr, 10, 64); err == nil {
			query.ReassignToDepartmentID = &reassignID
		}
	}

	return query
}

func (h *DepartmentHandler) toDepartmentResponse(dept *domain.Department) dto.DepartmentResponse {
	return dto.DepartmentResponse{
		ID:        dept.ID,
		Name:      dept.Name,
		ParentID:  dept.ParentID,
		CreatedAt: dept.CreatedAt,
	}
}

func (h *DepartmentHandler) toDepartmentResponseWithChildren(dept *domain.Department, includeEmployees bool) dto.DepartmentResponse {
	resp := dto.DepartmentResponse{
		ID:        dept.ID,
		Name:      dept.Name,
		ParentID:  dept.ParentID,
		CreatedAt: dept.CreatedAt,
	}

	if includeEmployees && len(dept.Employees) > 0 {
		resp.Employees = make([]dto.EmployeeResponse, len(dept.Employees))
		for i, emp := range dept.Employees {
			resp.Employees[i] = h.toEmployeeResponse(&emp)
		}
	}

	if len(dept.Children) > 0 {
		resp.Children = make([]dto.DepartmentResponse, len(dept.Children))
		for i, child := range dept.Children {
			resp.Children[i] = h.toDepartmentResponseWithChildren(&child, includeEmployees)
		}
	}

	return resp
}

func (h *DepartmentHandler) toEmployeeResponse(emp *domain.Employee) dto.EmployeeResponse {
	resp := dto.EmployeeResponse{
		ID:           emp.ID,
		DepartmentID: emp.DepartmentID,
		FullName:     emp.FullName,
		Position:     emp.Position,
		CreatedAt:    emp.CreatedAt,
	}

	if emp.HiredAt != nil {
		hiredAt := emp.HiredAt.Format("2006-01-02")
		resp.HiredAt = &hiredAt
	}

	return resp
}

func (h *DepartmentHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrDepartmentNotFound):
		h.respondError(w, http.StatusNotFound, "department not found", "")
	case errors.Is(err, domain.ErrEmployeeNotFound):
		h.respondError(w, http.StatusNotFound, "employee not found", "")
	case errors.Is(err, domain.ErrDuplicateDepartmentName):
		h.respondError(w, http.StatusConflict, "department with this name already exists", "")
	case errors.Is(err, domain.ErrSelfReference):
		h.respondError(w, http.StatusBadRequest, "department cannot be its own parent", "")
	case errors.Is(err, domain.ErrCyclicReference):
		h.respondError(w, http.StatusConflict, "moving department would create a cycle", "")
	case errors.Is(err, domain.ErrInvalidDeleteMode):
		h.respondError(w, http.StatusBadRequest, "invalid delete mode, use 'cascade' or 'reassign'", "")
	case errors.Is(err, domain.ErrReassignTargetRequired):
		h.respondError(w, http.StatusBadRequest, "reassign_to_department_id is required when mode is reassign", "")
	case errors.Is(err, domain.ErrReassignTargetNotFound):
		h.respondError(w, http.StatusNotFound, "target department for reassignment not found", "")
	case errors.Is(err, domain.ErrCannotReassignToSelf):
		h.respondError(w, http.StatusBadRequest, "cannot reassign to the same department being deleted", "")
	default:
		h.logger.Error("internal error", slog.Any("error", err))
		h.respondError(w, http.StatusInternalServerError, "internal server error", "")
	}
}

func (h *DepartmentHandler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", slog.Any("error", err))
	}
}

func (h *DepartmentHandler) respondError(w http.ResponseWriter, status int, errMsg, details string) {
	w.WriteHeader(status)
	resp := dto.ErrorResponse{Error: errMsg}
	if details != "" {
		resp.Message = details
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode error response", slog.Any("error", err))
	}
}
