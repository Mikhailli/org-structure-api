package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/org-structure-api/internal/middleware"
)

// Router настраивает маршруты API
type Router struct {
	mux     *http.ServeMux
	logger  *slog.Logger
	deptHandler *DepartmentHandler
}

// NewRouter создаёт новый роутер
func NewRouter(deptHandler *DepartmentHandler, logger *slog.Logger) *Router {
	return &Router{
		mux:         http.NewServeMux(),
		logger:      logger,
		deptHandler: deptHandler,
	}
}

// Setup настраивает все маршруты
func (r *Router) Setup() http.Handler {
	// Регистрируем обработчики
	r.mux.HandleFunc("/departments/", r.departmentsRouter)
	
	// Health check
	r.mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	
	// Применяем middleware
	handler := middleware.ContentType(r.mux)
	handler = middleware.Logger(r.logger)(handler)
	handler = middleware.Recoverer(r.logger)(handler)
	
	return handler
}

// departmentsRouter обрабатывает все запросы к /departments/
func (r *Router) departmentsRouter(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/departments")
	path = strings.Trim(path, "/")
	
	// POST /departments/ - создание подразделения
	if path == "" && req.Method == http.MethodPost {
		r.deptHandler.Create(w, req)
		return
	}
	
	// Разбираем путь: может быть {id} или {id}/employees
	parts := strings.Split(path, "/")
	
	if len(parts) == 1 && parts[0] != "" {
		// /departments/{id}
		switch req.Method {
		case http.MethodGet:
			r.deptHandler.GetByID(w, req)
		case http.MethodPatch:
			r.deptHandler.Update(w, req)
		case http.MethodDelete:
			r.deptHandler.Delete(w, req)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
		return
	}
	
	if len(parts) == 2 && parts[1] == "employees" {
		// /departments/{id}/employees/
		if req.Method == http.MethodPost {
			r.deptHandler.CreateEmployee(w, req)
			return
		}
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}
