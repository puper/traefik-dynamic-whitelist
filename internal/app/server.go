package app

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

//go:embed web/*
var webFiles embed.FS

type Server struct {
	cfg   Config
	store *Store
	now   func() time.Time
}

func NewServer(cfg Config) (*Server, error) {
	return &Server{
		cfg:   cfg,
		store: NewStore(cfg.StatePath, cfg.TraefikPath, cfg.TempDuration, TraefikConfig{IPStrategyDepth: cfg.TraefikIPDepth}),
		now:   time.Now,
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/info", s.auth(s.handleInfo))
	mux.HandleFunc("POST /api/add", s.auth(s.handleAdd))
	mux.HandleFunc("POST /api/delete", s.auth(s.handleDelete))

	staticFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AdminToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "凭证已失效，请重新输入")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	currentIPs, err := ClientIPs(r, s.cfg.ClientIPHeaders, s.cfg.TrustedProxyCIDR)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CLIENT_IP", "无法识别当前访问 IP")
		return
	}

	temporary, permanent, err := s.store.Info(s.now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STATE_ERROR", "读取白名单状态失败")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Result: infoResult{
			CurrentIP:    currentIPs[0],
			CurrentIPs:   currentIPs,
			TemporaryIPs: temporary,
			PermanentIPs: permanent,
		},
		Error: nil,
	})
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求内容不是有效 JSON")
		return
	}

	targetIPs, err := s.targetIPs(r, req.IP, req.IPs)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_IP", "授权 IP 格式无效")
		return
	}

	if err := s.store.AddMany(targetIPs, req.Type, s.now().UTC()); err != nil {
		if strings.Contains(err.Error(), "unsupported add type") {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "授权类型无效")
			return
		}
		writeError(w, http.StatusInternalServerError, "STATE_ERROR", "写入白名单状态失败")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Result: map[string]string{"status": "ok"},
		Error:  nil,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求内容不是有效 JSON")
		return
	}

	targetIP, err := s.targetIP(r, req.IP)
	if err != nil || strings.TrimSpace(req.IP) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_IP", "删除 IP 格式无效")
		return
	}

	if err := s.store.Delete(targetIP, s.now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "STATE_ERROR", "删除白名单状态失败")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Result: map[string]string{"status": "ok"},
		Error:  nil,
	})
}

func (s *Server) targetIPs(r *http.Request, requested string, requestedList []string) ([]string, error) {
	if strings.TrimSpace(requested) != "" {
		ip, err := s.targetIP(r, requested)
		if err != nil {
			return nil, err
		}
		return []string{ip}, nil
	}
	if len(requestedList) == 0 {
		return ClientIPs(r, s.cfg.ClientIPHeaders, s.cfg.TrustedProxyCIDR)
	}

	ips := make([]string, 0, len(requestedList))
	seen := map[string]bool{}
	for _, item := range requestedList {
		addr, err := netip.ParseAddr(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		ip := normalizeAddr(addr).String()
		if !seen[ip] {
			seen[ip] = true
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return nil, errors.New("empty ip list")
	}
	return ips, nil
}

func (s *Server) targetIP(r *http.Request, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return ClientIP(r, s.cfg.ClientIPHeaders, s.cfg.TrustedProxyCIDR)
	}

	addr, err := netip.ParseAddr(requested)
	if err != nil {
		return "", err
	}
	return normalizeAddr(addr).String(), nil
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, apiResponse{
		Result: nil,
		Error: &apiError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
